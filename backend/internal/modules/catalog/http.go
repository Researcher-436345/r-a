package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/queue"
	"github.com/centraluniversity/researcher/internal/platform/storage"
	"github.com/centraluniversity/researcher/internal/platform/throttle"
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
	// Limiter bounds the endpoints that fan out to external indexes; nil disables it (tests).
	Limiter *throttle.Limiter
}

// Per-user limits for resolving and discovery. Chat prefetch sends at most two
// resolves at a time, so a normal session stays far below them.
var (
	ruleResolveUser  = throttle.Rule{Name: "papers:resolve:user", Limit: 60, Window: time.Minute}
	ruleFullTextUser = throttle.Rule{Name: "papers:fulltext:user", Limit: 20, Window: time.Minute}
)

// allow writes 429 rate_limited (with Retry-After) when the user hit rule.
func (a API) allow(w http.ResponseWriter, r *http.Request, rule throttle.Rule) bool {
	if a.Limiter == nil {
		return true
	}
	allowed, after := a.Limiter.Hit(r.Context(), rule, resolveRateSubject(r))
	if allowed {
		return true
	}
	if after > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(after.Seconds()))))
	}
	httpx.ErrorCode(w, http.StatusTooManyRequests, "rate_limited", "Too many requests, try again in a minute")
	return false
}

// Gateway appends the actual peer to X-Forwarded-For. Use that last address,
// never the caller-supplied first address; internal services are not public.
func resolveRateSubject(r *http.Request) string {
	if id := identity.UserID(r); id != uuid.Nil {
		return id.String()
	}
	peer := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		peer = strings.TrimSpace(parts[len(parts)-1])
	}
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	return "guest:" + peer
}

func (a API) store() Store { return Store{DB: a.DB} }

func (a API) Mount(r chi.Router) {
	r.Post("/papers/arxiv", a.addArxiv)
	r.Post("/papers/arxiv/open", a.openArxiv)
	r.Post("/papers/doi", a.addDOI)
	r.Post("/papers/from-url", a.addFromURL)
	r.Post("/papers/upload", a.upload)
	r.Get("/papers/{paperID}", a.getPaper)
	r.Get("/papers/{paperID}/pdf-url", a.pdfURL)
	r.Get("/papers/{paperID}/pdf", a.pdfStream)
	r.Post("/papers/{paperID}/retry-pdf", a.retryPDF)
	r.Post("/papers/{paperID}/find-fulltext", a.findFullText)
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
	allowed, e := a.canAccessPaper(r.Context(), identity.UserID(r), id)
	if e != nil || !allowed {
		httpx.Error(w, 404, "Paper not found")
		return id, false
	}
	return id, true
}

func (a API) canAccessPaper(ctx context.Context, userID, paperID uuid.UUID) (bool, error) {
	if userID == uuid.Nil {
		version, err := a.store().LatestVersion(ctx, paperID)
		return err == nil && publicVersion(version), err
	}
	inLibrary, err := a.Membership.Has(ctx, userID, paperID)
	if err != nil || inLibrary {
		return inLibrary, err
	}
	return a.store().IsPublic(ctx, paperID)
}

// A public identifier does not make a user-uploaded PDF public.
func publicVersion(v Version) bool {
	return v.Source == "arxiv" || v.Source == "doi" || v.Source == "web_pdf" || v.Source == "openalex"
}

func addToLibrary(value *bool) bool {
	return value == nil || *value
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
	a.arxiv(w, r, false)
}

// Opening a public paper never modifies a user library, regardless of the body.
func (a API) openArxiv(w http.ResponseWriter, r *http.Request) {
	a.arxiv(w, r, true)
}

func (a API) arxiv(w http.ResponseWriter, r *http.Request, openOnly bool) {
	var b struct {
		ArxivID      string `json:"arxiv_id"`
		AddToLibrary *bool  `json:"add_to_library"`
	}
	if !httpx.DecodeJSON(w, r, &b) || !a.allow(w, r, ruleResolveUser) {
		return
	}
	raw, e := NormalizeArxivID(b.ArxivID)
	if e != nil {
		httpx.Error(w, 400, e.Error())
		return
	}
	id := CanonicalArxivID(raw)
	paperID, e := a.addArxivPaper(r.Context(), identity.UserID(r), id, !openOnly && addToLibrary(b.AddToLibrary))
	if e != nil {
		var sourceErr *sourceFetchError
		if errors.As(e, &sourceErr) {
			httpx.Error(w, 502, sourceErr.Error())
			return
		}
		httpx.Error(w, 500, e.Error())
		return
	}
	if identity.UserID(r) == uuid.Nil {
		allowed, err := a.canAccessPaper(r.Context(), uuid.Nil, paperID)
		if err != nil || !allowed {
			httpx.Error(w, 404, "Paper not found")
			return
		}
	}
	a.paperResponse(w, r, paperID, 201)
}

func (a API) addDOI(w http.ResponseWriter, r *http.Request) {
	var b struct {
		DOI          string `json:"doi"`
		AddToLibrary *bool  `json:"add_to_library"`
	}
	if !httpx.DecodeJSON(w, r, &b) || !a.allow(w, r, ruleResolveUser) {
		return
	}
	doi, e := NormalizeDOI(b.DOI)
	if e != nil {
		httpx.Error(w, 400, e.Error())
		return
	}
	paperID, e := a.addOpenDOIPaper(r.Context(), identity.UserID(r), doi, "", addToLibrary(b.AddToLibrary))
	if e != nil {
		// Container DOIs answer 422 not_a_paper, unknown DOIs 422 not_found.
		writeResolveError(w, e, false)
		return
	}
	a.paperResponse(w, r, paperID, 201)
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

// Codes the reader branches on (contract B3). Only pdf_processing means
// "keep polling"; pdf_unavailable means no PDF appears without user action.
const (
	pdfCodeProcessing  = "pdf_processing"
	pdfCodeUnavailable = "pdf_unavailable"
)

type pdfState struct {
	HTTPStatus int
	Code       string
	Detail     string
}

// pdfStateFor maps the latest paper version to the pdf-url / pdf response.
func pdfStateFor(v Version) pdfState {
	switch {
	case v.Status == "ready" && v.PDFKey != nil:
		return pdfState{HTTPStatus: http.StatusOK}
	case v.Status == "processing":
		return pdfState{HTTPStatus: http.StatusConflict, Code: pdfCodeProcessing, Detail: "PDF is still processing"}
	case v.Status == "failed":
		return pdfState{
			HTTPStatus: http.StatusUnprocessableEntity,
			Code:       pdfCodeUnavailable,
			// The worker stores a user-readable reason (e.g. which host refused).
			Detail: orString(v.ErrorMessage, "PDF processing failed"),
		}
	default:
		// Metadata-only versions (doi, openalex) are "ready" without a file.
		return pdfState{
			HTTPStatus: http.StatusUnprocessableEntity,
			Code:       pdfCodeUnavailable,
			Detail:     "Full text is not available in open access for this paper",
		}
	}
}

// readyPDFVersion writes the B3 error response and returns ok=false unless
// the latest version has a stored PDF.
func (a API) readyPDFVersion(w http.ResponseWriter, r *http.Request) (uuid.UUID, Version, bool) {
	id, ok := a.requirePaper(w, r)
	if !ok {
		return id, Version{}, false
	}
	v, e := a.store().LatestVersion(r.Context(), id)
	if errors.Is(e, pgx.ErrNoRows) {
		httpx.Error(w, 404, "PDF not available yet")
		return id, v, false
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return id, v, false
	}
	if st := pdfStateFor(v); st.HTTPStatus != http.StatusOK {
		httpx.ErrorCode(w, st.HTTPStatus, st.Code, st.Detail)
		return id, v, false
	}
	return id, v, true
}

func (a API) pdfURL(w http.ResponseWriter, r *http.Request) {
	id, _, ok := a.readyPDFVersion(w, r)
	if !ok {
		return
	}
	httpx.JSON(w, 200, map[string]any{
		"url":        "/papers/" + id.String() + "/pdf",
		"expires_in": 0,
		"status":     "ready",
		"source":     "api",
	})
}

func (a API) pdfStream(w http.ResponseWriter, r *http.Request) {
	_, v, ok := a.readyPDFVersion(w, r)
	if !ok {
		return
	}
	if a.Storage == nil {
		httpx.Error(w, 502, "failed to read PDF from storage")
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
	if errors.Is(e, pgx.ErrNoRows) {
		httpx.Error(w, 400, "No PDF version found")
		return
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	task, refusal := retryTaskFor(v)
	if refusal != nil {
		httpx.ErrorCode(w, refusal.HTTPStatus, refusal.Code, refusal.Detail)
		return
	}
	if task == "" {
		a.paperResponse(w, r, id, 200)
		return
	}
	if a.Queue == nil {
		httpx.Error(w, 503, "Background queue is unavailable")
		return
	}
	previous := v
	v.Status = "processing"
	v.ErrorMessage = nil
	if e = a.store().UpdateVersion(r.Context(), v); e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if e = queue.Enqueue(a.Queue, task, v.ID.String()); e != nil {
		// No job behind it: restore the old state instead of a "processing" the reader polls forever.
		_ = a.store().UpdateVersion(r.Context(), previous)
		httpx.Error(w, 503, "Failed to queue PDF retry: "+e.Error())
		return
	}
	a.paperResponse(w, r, id, 200)
}

// retryTaskFor picks the worker task that rebuilds the latest version's PDF.
// task=="" with refusal==nil means the PDF is already there. A refusal means
// there is nothing to re-download, so the client should use find-fulltext.
func retryTaskFor(v Version) (task string, refusal *pdfState) {
	if v.Status == "ready" && v.PDFKey != nil {
		return "", nil
	}
	switch v.Source {
	case "upload":
		if v.PDFKey != nil {
			return queue.FinalizeUploadedPDF, nil
		}
	case "arxiv", "web_pdf":
		if v.SourceURL != nil && strings.TrimSpace(*v.SourceURL) != "" {
			// Same task name as requeueRemotePDF in url.go.
			return queue.ProcessArxivPDF, nil
		}
	}
	return "", &pdfState{
		HTTPStatus: http.StatusUnprocessableEntity,
		Code:       pdfCodeUnavailable,
		Detail:     "This paper has no PDF source to retry. Use Find full text (POST /papers/" + v.PaperID.String() + "/find-fulltext) to search open-access sources.",
	}
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

// RequirePaper is used by sibling modules (assistant, annotations) via the shared access check.
func (a API) RequirePaper(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return a.requirePaper(w, r)
}
