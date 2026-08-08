package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/queue"
	"github.com/centraluniversity/researcher/internal/platform/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	DB         *pgxpool.Pool
	Storage    *storage.Client
	Queue      *asynq.Client
	Membership Membership
}

func (a API) store() Store { return Store{DB: a.DB} }

func (a API) Mount(r chi.Router) {
	r.Post("/papers/arxiv", a.addArxiv)
	r.Post("/papers/doi", a.addDOI)
	r.Post("/papers/upload", a.upload)
	r.Get("/papers/{paperID}", a.getPaper)
	r.Get("/papers/{paperID}/pdf-url", a.pdfURL)
	r.Get("/papers/{paperID}/pdf", a.pdfStream)
	r.Post("/papers/{paperID}/retry-pdf", a.retryPDF)
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, e := uuid.Parse(chi.URLParam(r, key))
	if e != nil {
		httpx.Error(w, 404, "Not found")
		return uuid.Nil, false
	}
	return id, true
}

func ptr(s string) *string { return &s }

func (a API) requirePaper(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := parseID(w, r, "paperID")
	if !ok {
		return id, false
	}
	in, e := a.Membership.Has(r.Context(), identity.UserID(r), id)
	if e != nil || !in {
		httpx.Error(w, 404, "Paper not found")
		return id, false
	}
	return id, true
}

func (a API) paperResponse(w http.ResponseWriter, r *http.Request, id uuid.UUID, status int) {
	p, e := a.store().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, status, p)
}

func (a API) addArxiv(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ArxivID string `json:"arxiv_id"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	raw, e := NormalizeArxivID(b.ArxivID)
	if e != nil {
		httpx.Error(w, 400, e.Error())
		return
	}
	id := CanonicalArxivID(raw)
	p, e := a.store().FindByArxiv(r.Context(), id)
	if e == nil && p != nil {
		if e = a.Membership.Add(r.Context(), identity.UserID(r), p.ID); e != nil {
			httpx.Error(w, 500, e.Error())
			return
		}
		a.paperResponse(w, r, p.ID, 201)
		return
	}
	m, e := FetchArxivMetadata(r.Context(), id)
	if e != nil {
		httpx.Error(w, 502, "Failed to fetch arXiv metadata: "+e.Error())
		return
	}
	venue := "arXiv"
	created, e := a.store().CreatePaper(r.Context(), m.Title, ptr(m.Abstract), m.Year, &venue, nil, &m.ArxivID)
	p = &created
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if e = a.store().AttachAuthors(r.Context(), p.ID, m.Authors); e == nil {
		v, e2 := a.store().CreateVersion(r.Context(), p.ID, 1, "arxiv", &m.PDFURL, nil, nil, nil, "processing")
		e = e2
		if e == nil {
			e = a.Membership.Add(r.Context(), identity.UserID(r), p.ID)
			if e == nil && a.Queue != nil {
				e = queue.Enqueue(a.Queue, queue.ProcessArxivPDF, v.ID.String())
			}
		}
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	a.paperResponse(w, r, p.ID, 201)
}

func (a API) addDOI(w http.ResponseWriter, r *http.Request) {
	var b struct {
		DOI string `json:"doi"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	doi, e := NormalizeDOI(b.DOI)
	if e != nil {
		httpx.Error(w, 400, e.Error())
		return
	}
	p, e := a.store().FindByDOI(r.Context(), doi)
	if e == nil && p != nil {
		if e = a.Membership.Add(r.Context(), identity.UserID(r), p.ID); e != nil {
			httpx.Error(w, 500, e.Error())
			return
		}
		a.paperResponse(w, r, p.ID, 201)
		return
	}
	m, e := FetchCrossrefMetadata(r.Context(), doi)
	if e != nil {
		httpx.Error(w, 502, "Failed to fetch Crossref metadata: "+e.Error())
		return
	}
	created, e := a.store().CreatePaper(r.Context(), m.Title, m.Abstract, m.Year, m.Venue, &m.DOI, nil)
	p = &created
	if e == nil {
		e = a.store().AttachAuthors(r.Context(), p.ID, m.Authors)
	}
	if e == nil {
		sourceURL := "https://doi.org/" + m.DOI
		_, e = a.store().CreateVersion(r.Context(), p.ID, 1, "doi", &sourceURL, nil, nil, nil, "ready")
	}
	if e == nil {
		e = a.Membership.Add(r.Context(), identity.UserID(r), p.ID)
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	a.paperResponse(w, r, p.ID, 201)
}

func (a API) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	if e := r.ParseMultipartForm(50 << 20); e != nil {
		httpx.Error(w, 400, "File too large (max 50MB)")
		return
	}
	f, h, e := r.FormFile("file")
	if e != nil {
		httpx.Error(w, 400, "Only PDF files are supported")
		return
	}
	defer f.Close()
	data, e := httpx.ReadBody(w, &http.Request{Body: f}, 50<<20)
	if e != nil || len(data) == 0 {
		httpx.Error(w, 400, map[bool]string{true: "Empty file", false: "File too large (max 50MB)"}[len(data) == 0])
		return
	}
	if !strings.HasSuffix(strings.ToLower(h.Filename), ".pdf") && h.Header.Get("Content-Type") != "application/pdf" && h.Header.Get("Content-Type") != "application/octet-stream" {
		httpx.Error(w, 400, "Only PDF files are supported")
		return
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	fallback := strings.TrimSuffix(filepath.Base(h.Filename), filepath.Ext(h.Filename))
	meta := ExtractPDFInfo(data)
	title, abstract, year, venue, doi, arxivID, authors := a.enrichUploadMeta(r.Context(), meta, fallback)
	existing, e := a.store().FindVersionBySHA(r.Context(), sha)
	if e == nil && existing != nil {
		// Re-upload of same PDF: repair missing/junk metadata.
		if cur, err := a.store().GetPaperOut(r.Context(), existing.PaperID); err == nil {
			needsRepair := !isUsefulTitle(cur.Title) || cur.Title == "Untitled PDF" || (arxivID != nil && cur.ArxivID == nil)
			if needsRepair && title != "" {
				_ = a.store().UpdatePaperMeta(r.Context(), existing.PaperID, title, abstract, year, venue, doi, arxivID)
				if len(authors) > 0 {
					_ = a.store().AttachAuthors(r.Context(), existing.PaperID, authors)
				}
			}
		}
		if e = a.Membership.Add(r.Context(), identity.UserID(r), existing.PaperID); e == nil {
			a.paperResponse(w, r, existing.PaperID, 201)
			return
		}
	}
	p, e := a.store().CreatePaper(r.Context(), title, abstract, year, venue, doi, arxivID)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if len(authors) > 0 {
		_ = a.store().AttachAuthors(r.Context(), p.ID, authors)
	}
	key := storage.PDFKey(p.ID.String(), uuid.NewString())
	if a.Storage == nil {
		httpx.Error(w, 502, "Failed to store PDF: storage unavailable")
		return
	}
	if e = a.Storage.Upload(r.Context(), key, data); e == nil {
		size := int64(len(data))
		v, e2 := a.store().CreateVersion(r.Context(), p.ID, 1, "upload", nil, &key, &sha, &size, "processing")
		e = e2
		if e == nil {
			e = a.Membership.Add(r.Context(), identity.UserID(r), p.ID)
			if e == nil && a.Queue != nil {
				e = queue.Enqueue(a.Queue, queue.FinalizeUploadedPDF, v.ID.String())
			}
		}
	}
	if e != nil {
		httpx.Error(w, 502, "Failed to store PDF: "+e.Error())
		return
	}
	a.paperResponse(w, r, p.ID, 201)
}

func (a API) getPaper(w http.ResponseWriter, r *http.Request) {
	id, ok := a.requirePaper(w, r)
	if ok {
		a.paperResponse(w, r, id, 200)
	}
}

func (a API) pdfURL(w http.ResponseWriter, r *http.Request) {
	id, ok := a.requirePaper(w, r)
	if !ok {
		return
	}
	v, e := a.store().LatestVersion(r.Context(), id)
	if e == pgx.ErrNoRows {
		httpx.Error(w, 404, "PDF not available yet")
		return
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if v.Status == "ready" && v.PDFKey != nil {
		httpx.JSON(w, 200, map[string]any{
			"url":        "/papers/" + id.String() + "/pdf",
			"expires_in": 0,
			"status":     "ready",
			"source":     "api",
		})
		return
	}
	if v.Status == "failed" {
		httpx.Error(w, 409, orString(v.ErrorMessage, "PDF processing failed"))
		return
	}
	httpx.Error(w, 409, "PDF is still processing")
}

func (a API) pdfStream(w http.ResponseWriter, r *http.Request) {
	id, ok := a.requirePaper(w, r)
	if !ok {
		return
	}
	v, e := a.store().LatestVersion(r.Context(), id)
	if e == pgx.ErrNoRows {
		httpx.Error(w, 404, "PDF not available yet")
		return
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if v.Status == "failed" {
		httpx.Error(w, 409, orString(v.ErrorMessage, "PDF processing failed"))
		return
	}
	if v.Status != "ready" || v.PDFKey == nil {
		httpx.Error(w, 409, "PDF is still processing")
		return
	}
	data, err := a.Storage.Download(r.Context(), *v.PDFKey)
	if err != nil {
		httpx.Error(w, 502, "failed to read PDF from storage")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}

func (a API) retryPDF(w http.ResponseWriter, r *http.Request) {
	id, ok := a.requirePaper(w, r)
	if !ok {
		return
	}
	v, e := a.store().LatestVersion(r.Context(), id)
	if e != nil {
		httpx.Error(w, 400, "No PDF version found")
		return
	}
	if v.Status != "ready" || v.PDFKey == nil {
		v.Status = "processing"
		v.ErrorMessage = nil
		e = a.store().UpdateVersion(r.Context(), v)
		if e == nil && a.Queue != nil {
			typ := queue.ProcessArxivPDF
			if v.Source == "upload" {
				typ = queue.FinalizeUploadedPDF
			} else if v.Source != "arxiv" {
				httpx.Error(w, 400, "Cannot retry this PDF source")
				return
			}
			e = queue.Enqueue(a.Queue, typ, v.ID.String())
		}
		if e != nil {
			httpx.Error(w, 500, e.Error())
			return
		}
	}
	a.paperResponse(w, r, id, 200)
}

func orString(p *string, f string) string {
	if p == nil || *p == "" {
		return f
	}
	return *p
}

func (a API) enrichUploadMeta(ctx context.Context, meta PDFInfo, fallback string) (title string, abstract *string, year *int, venue, doi, arxivID *string, authors []string) {
	title = meta.Title
	authors = meta.Authors
	if meta.ArxivID != "" {
		id := CanonicalArxivID(meta.ArxivID)
		arxivID = &id
		if m, err := FetchArxivMetadata(ctx, id); err == nil {
			if m.Title != "" {
				title = m.Title
			}
			if m.Abstract != "" {
				abstract = ptr(m.Abstract)
			}
			year = m.Year
			v := "arXiv"
			venue = &v
			if len(m.Authors) > 0 {
				authors = m.Authors
			}
		}
	}
	if title == "" {
		title = cleanFilenameFallback(fallback)
	}
	return
}

// RequirePaper is used by sibling modules (assistant, annotations) via shared membership check.
func (a API) RequirePaper(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return a.requirePaper(w, r)
}
