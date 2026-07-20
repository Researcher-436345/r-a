package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/auth"
	"github.com/centraluniversity/researcher/internal/config"
	"github.com/centraluniversity/researcher/internal/db"
	"github.com/centraluniversity/researcher/internal/httpx"
	"github.com/centraluniversity/researcher/internal/models"
	"github.com/centraluniversity/researcher/internal/queue"
	"github.com/centraluniversity/researcher/internal/services"
	"github.com/centraluniversity/researcher/internal/storage"
	"github.com/centraluniversity/researcher/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	Config  config.Config
	DB      *pgxpool.Pool
	Storage *storage.Client
	Queue   *asynq.Client
	Redis   *redis.Client
}

func (s Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{AllowedOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"}, AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"}, AllowedHeaders: []string{"Authorization", "Content-Type"}, AllowCredentials: true, MaxAge: 300}))
	r.Get("/", s.root)
	r.Get("/health", s.health)
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", s.register)
		r.Post("/login", s.login)
		r.Post("/refresh", s.refresh)
		r.With(s.requireUser).Get("/me", s.me)
	})
	r.Group(func(r chi.Router) {
		r.Use(s.requireUser)
		r.Post("/papers/arxiv", s.addArxiv)
		r.Post("/papers/doi", s.addDOI)
		r.Post("/papers/upload", s.upload)
		r.Get("/papers/{paperID}", s.getPaper)
		r.Get("/papers/{paperID}/pdf-url", s.pdfURL)
		r.Get("/papers/{paperID}/pdf", s.pdfStream)
		r.Post("/papers/{paperID}/retry-pdf", s.retryPDF)
		r.Post("/papers/{paperID}/chat", s.chat)
		r.Post("/papers/{paperID}/explain", s.explain)
		r.Post("/papers/{paperID}/translate", s.translate)
		r.Get("/library", s.library)
		r.Patch("/library/{paperID}", s.patchLibrary)
		r.Delete("/library/{paperID}", s.deleteLibrary)
		r.Get("/papers/{paperID}/annotations", s.listAnnotations)
		r.Post("/papers/{paperID}/annotations", s.createAnnotation)
		r.Patch("/annotations/{annotationID}", s.patchAnnotation)
		r.Delete("/annotations/{annotationID}", s.deleteAnnotation)
		r.Get("/feed/trending", s.trending)
	})
	return r
}
func (s Server) users() store.Users   { return store.Users{DB: s.DB} }
func (s Server) papers() store.Papers { return store.Papers{DB: s.DB} }
func (s Server) root(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, 200, map[string]string{"service": s.Config.AppName, "docs": "/docs", "health": "/health", "db_reachable": strconv.FormatBool(db.Check(r.Context(), s.DB))})
}
func (s Server) health(w http.ResponseWriter, r *http.Request) {
	ok := db.Check(r.Context(), s.DB)
	m := "skipped"
	if ok {
		if db.MigrationsOK(r.Context(), s.DB) {
			m = "ok"
		} else {
			m = "error"
		}
	}
	status := "degraded"
	if ok && m == "ok" {
		status = "ok"
	}
	httpx.JSON(w, 200, map[string]string{"status": status, "db": map[bool]string{true: "ok", false: "error"}[ok], "migrations": m})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (s Server) register(w http.ResponseWriter, r *http.Request) {
	var b credentials
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	if b.Email == "" || len(b.Password) < 8 {
		httpx.Error(w, 400, "Invalid email or password")
		return
	}
	hash, e := auth.HashPassword(b.Password)
	if e != nil {
		httpx.Error(w, 500, "Failed to hash password")
		return
	}
	u, e := s.users().CreateUser(r.Context(), b.Email, hash)
	if e != nil {
		if strings.Contains(e.Error(), "duplicate") {
			httpx.Error(w, 409, "Email already registered")
			return
		}
		httpx.Error(w, 500, "Failed to create user")
		return
	}
	s.tokens(w, u.ID, 201)
}
func (s Server) login(w http.ResponseWriter, r *http.Request) {
	var b credentials
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	u, e := s.users().GetByEmail(r.Context(), strings.ToLower(strings.TrimSpace(b.Email)))
	if e != nil || !auth.CheckPassword(u.PasswordHash, b.Password) {
		httpx.Error(w, 401, "Incorrect email or password")
		return
	}
	s.tokens(w, u.ID, 200)
}
func (s Server) refresh(w http.ResponseWriter, r *http.Request) {
	var b refreshReq
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	id, e := auth.ParseToken(s.Config.JWTSecret, b.RefreshToken, "refresh")
	if e != nil {
		httpx.Error(w, 401, "Invalid or expired refresh token")
		return
	}
	if _, e = s.users().GetByID(r.Context(), id); e != nil {
		httpx.Error(w, 401, "User not found")
		return
	}
	s.tokens(w, id, 200)
}
func (s Server) tokens(w http.ResponseWriter, id uuid.UUID, status int) {
	t, e := auth.IssueTokens(s.Config.JWTSecret, id, s.Config.AccessTokenTTL, s.Config.RefreshTokenTTL)
	if e != nil {
		httpx.Error(w, 500, "Failed to issue token")
		return
	}
	httpx.JSON(w, status, t)
}
func (s Server) me(w http.ResponseWriter, r *http.Request) {
	u, err := s.users().GetByID(r.Context(), userID(r))
	if err != nil {
		httpx.Error(w, 401, "User not found")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": u.ID, "email": u.Email, "created_at": u.CreatedAt})
}
func (s Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if v == r.Header.Get("Authorization") || v == "" {
			httpx.Error(w, 401, "Not authenticated")
			return
		}
		id, e := auth.ParseToken(s.Config.JWTSecret, v, "access")
		if e != nil {
			httpx.Error(w, 401, "Could not validate credentials")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, id)))
	})
}

type userKey struct{}

func userID(r *http.Request) uuid.UUID { return r.Context().Value(userKey{}).(uuid.UUID) }
func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, e := uuid.Parse(chi.URLParam(r, key))
	if e != nil {
		httpx.Error(w, 404, "Not found")
		return uuid.Nil, false
	}
	return id, true
}
func ptr(s string) *string { return &s }
func (s Server) addArxiv(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ArxivID string `json:"arxiv_id"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	raw, e := services.NormalizeArxivID(b.ArxivID)
	if e != nil {
		httpx.Error(w, 400, e.Error())
		return
	}
	id := services.CanonicalArxivID(raw)
	p, e := s.papers().FindByArxiv(r.Context(), id)
	if e == nil && p != nil {
		out, e := s.papers().AddToLibrary(r.Context(), userID(r), p.ID)
		_ = out
		if e != nil {
			httpx.Error(w, 500, e.Error())
			return
		}
		s.paperResponse(w, r, p.ID, 201)
		return
	}
	m, e := services.FetchArxivMetadata(r.Context(), id)
	if e != nil {
		httpx.Error(w, 502, "Failed to fetch arXiv metadata: "+e.Error())
		return
	}
	venue := "arXiv"
	created, e := s.papers().CreatePaper(r.Context(), m.Title, ptr(m.Abstract), m.Year, &venue, nil, &m.ArxivID)
	p = &created
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if e = s.papers().AttachAuthors(r.Context(), p.ID, m.Authors); e == nil {
		v, e := s.papers().CreateVersion(r.Context(), p.ID, 1, "arxiv", &m.PDFURL, nil, nil, nil, "processing")
		if e == nil {
			_, e = s.papers().AddToLibrary(r.Context(), userID(r), p.ID)
			if e == nil && s.Queue != nil {
				e = queue.Enqueue(s.Queue, queue.ProcessArxivPDF, v.ID.String())
			}
		}
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	s.paperResponse(w, r, p.ID, 201)
}
func (s Server) addDOI(w http.ResponseWriter, r *http.Request) {
	var b struct {
		DOI string `json:"doi"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	doi, e := services.NormalizeDOI(b.DOI)
	if e != nil {
		httpx.Error(w, 400, e.Error())
		return
	}
	p, e := s.papers().FindByDOI(r.Context(), doi)
	if e == nil && p != nil {
		_, e = s.papers().AddToLibrary(r.Context(), userID(r), p.ID)
		if e != nil {
			httpx.Error(w, 500, e.Error())
			return
		}
		s.paperResponse(w, r, p.ID, 201)
		return
	}
	m, e := services.FetchCrossrefMetadata(r.Context(), doi)
	if e != nil {
		httpx.Error(w, 502, "Failed to fetch Crossref metadata: "+e.Error())
		return
	}
	created, e := s.papers().CreatePaper(r.Context(), m.Title, m.Abstract, m.Year, m.Venue, &m.DOI, nil)
	p = &created
	if e == nil {
		e = s.papers().AttachAuthors(r.Context(), p.ID, m.Authors)
	}
	if e == nil {
		sourceURL := "https://doi.org/" + m.DOI
		_, e = s.papers().CreateVersion(r.Context(), p.ID, 1, "doi", &sourceURL, nil, nil, nil, "ready")
	}
	if e == nil {
		_, e = s.papers().AddToLibrary(r.Context(), userID(r), p.ID)
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	s.paperResponse(w, r, p.ID, 201)
}
func (s Server) upload(w http.ResponseWriter, r *http.Request) {
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
	existing, e := s.papers().FindVersionBySHA(r.Context(), sha)
	if e == nil && existing != nil {
		_, e = s.papers().AddToLibrary(r.Context(), userID(r), existing.PaperID)
		if e == nil {
			s.paperResponse(w, r, existing.PaperID, 201)
			return
		}
	}
	title := services.ExtractTitleFromPDF(data, strings.TrimSuffix(filepath.Base(h.Filename), filepath.Ext(h.Filename)))
	p, e := s.papers().CreatePaper(r.Context(), title, nil, nil, nil, nil, nil)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	key := storage.PDFKey(p.ID.String(), uuid.NewString())
	if s.Storage == nil {
		httpx.Error(w, 502, "Failed to store PDF: storage unavailable")
		return
	}
	if e = s.Storage.Upload(r.Context(), key, data); e == nil {
		size := int64(len(data))
		v, e := s.papers().CreateVersion(r.Context(), p.ID, 1, "upload", nil, &key, &sha, &size, "processing")
		if e == nil {
			_, e = s.papers().AddToLibrary(r.Context(), userID(r), p.ID)
			if e == nil && s.Queue != nil {
				e = queue.Enqueue(s.Queue, queue.FinalizeUploadedPDF, v.ID.String())
			}
		}
	}
	if e != nil {
		httpx.Error(w, 502, "Failed to store PDF: "+e.Error())
		return
	}
	s.paperResponse(w, r, p.ID, 201)
}
func (s Server) paperResponse(w http.ResponseWriter, r *http.Request, id uuid.UUID, status int) {
	p, e := s.papers().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, status, p)
}
func (s Server) requirePaper(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := parseID(w, r, "paperID")
	if !ok {
		return id, false
	}
	in, e := s.papers().EnsureInLibrary(r.Context(), userID(r), id)
	if e != nil || !in {
		httpx.Error(w, 404, "Paper not found")
		return id, false
	}
	return id, true
}
func (s Server) getPaper(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if ok {
		s.paperResponse(w, r, id, 200)
	}
}
func (s Server) pdfURL(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if !ok {
		return
	}
	v, e := s.papers().LatestVersion(r.Context(), id)
	if e == pgx.ErrNoRows {
		httpx.Error(w, 404, "PDF not available yet")
		return
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if v.Status == "ready" && v.PDFKey != nil {
		// Браузер ходит в API, не в MinIO напрямую (порты MinIO часто перехватывает Cursor).
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

func (s Server) pdfStream(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if !ok {
		return
	}
	v, e := s.papers().LatestVersion(r.Context(), id)
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
	data, err := s.Storage.Download(r.Context(), *v.PDFKey)
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
func (s Server) retryPDF(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if !ok {
		return
	}
	v, e := s.papers().LatestVersion(r.Context(), id)
	if e != nil {
		httpx.Error(w, 400, "No PDF version found")
		return
	}
	if v.Status != "ready" || v.PDFKey == nil {
		v.Status = "processing"
		v.ErrorMessage = nil
		e = s.papers().UpdateVersion(r.Context(), v)
		if e == nil && s.Queue != nil {
			typ := queue.ProcessArxivPDF
			if v.Source == "upload" {
				typ = queue.FinalizeUploadedPDF
			} else if v.Source != "arxiv" {
				httpx.Error(w, 400, "Cannot retry this PDF source")
				return
			}
			e = queue.Enqueue(s.Queue, typ, v.ID.String())
		}
		if e != nil {
			httpx.Error(w, 500, e.Error())
			return
		}
	}
	s.paperResponse(w, r, id, 200)
}
func (s Server) chat(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if !ok {
		return
	}
	var b struct {
		Message     string `json:"message"`
		ContextText string `json:"context_text"`
	}
	if !httpx.DecodeJSON(w, r, &b) || strings.TrimSpace(b.Message) == "" {
		return
	}
	p, e := s.papers().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 404, "Paper not found")
		return
	}
	httpx.JSON(w, 200, map[string]string{"reply": (services.LLM{Config: s.Config}).Chat(r.Context(), p, b.Message, b.ContextText)})
}
func (s Server) explain(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if !ok {
		return
	}
	var b struct {
		Text     string `json:"text"`
		Question string `json:"question"`
	}
	if !httpx.DecodeJSON(w, r, &b) || strings.TrimSpace(b.Text) == "" {
		return
	}
	p, e := s.papers().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 404, "Paper not found")
		return
	}
	httpx.JSON(w, 200, map[string]string{"reply": (services.LLM{Config: s.Config}).Explain(r.Context(), p, b.Text, b.Question)})
}
func (s Server) translate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if !ok {
		return
	}
	var b struct {
		Text       string `json:"text"`
		TargetLang string `json:"target_lang"`
	}
	if !httpx.DecodeJSON(w, r, &b) || strings.TrimSpace(b.Text) == "" {
		return
	}
	if b.TargetLang == "" {
		b.TargetLang = "ru"
	}
	p, e := s.papers().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 404, "Paper not found")
		return
	}
	t, d := services.Translate(r.Context(), services.LLM{Config: s.Config}, p, b.Text, b.TargetLang)
	httpx.JSON(w, 200, map[string]any{"translation": t, "detected_source": d})
}
func (s Server) library(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var status *string
	if x := r.URL.Query().Get("status"); x != "" {
		status = &x
	}
	items, total, e := s.papers().ListLibrary(r.Context(), userID(r), page, limit, status)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": items, "page": page, "limit": limit, "total": total})
}
func (s Server) patchLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "paperID")
	if !ok {
		return
	}
	var b struct {
		Status   *string `json:"status"`
		Favorite *bool   `json:"favorite"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.Status != nil && *b.Status != "unread" && *b.Status != "reading" && *b.Status != "read" {
		httpx.Error(w, 400, "Invalid status")
		return
	}
	out, e := s.papers().PatchLibrary(r.Context(), userID(r), id, b.Status, b.Favorite)
	if e == pgx.ErrNoRows {
		httpx.Error(w, 404, "Library item not found")
		return
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 200, out)
}
func (s Server) deleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "paperID")
	if !ok {
		return
	}
	deleted, e := s.papers().DeleteLibrary(r.Context(), userID(r), id)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if !deleted {
		httpx.Error(w, 404, "Library item not found")
		return
	}
	w.WriteHeader(204)
}
func (s Server) listAnnotations(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if !ok {
		return
	}
	out, e := s.papers().ListAnnotations(r.Context(), userID(r), id)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 200, out)
}
func (s Server) createAnnotation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.requirePaper(w, r)
	if !ok {
		return
	}
	var b struct {
		Page         int                    `json:"page"`
		Rect         *models.AnnotationRect `json:"rect"`
		SelectedText string                 `json:"selected_text"`
		Note         string                 `json:"note"`
		Color        string                 `json:"color"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.Page < 1 || strings.TrimSpace(b.SelectedText) == "" {
		httpx.Error(w, 400, "Invalid annotation")
		return
	}
	if b.Color == "" {
		b.Color = "#facc15"
	}
	a, e := s.papers().CreateAnnotation(r.Context(), userID(r), id, b.Page, b.Rect, strings.TrimSpace(b.SelectedText), strings.TrimSpace(b.Note), b.Color)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 201, a)
}
func (s Server) patchAnnotation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "annotationID")
	if !ok {
		return
	}
	var b struct {
		Note string `json:"note"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	a, e := s.papers().PatchAnnotation(r.Context(), userID(r), id, strings.TrimSpace(b.Note))
	if errors.Is(e, pgx.ErrNoRows) {
		httpx.Error(w, 404, "Annotation not found")
		return
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 200, a)
}
func (s Server) deleteAnnotation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "annotationID")
	if !ok {
		return
	}
	deleted, e := s.papers().DeleteAnnotation(r.Context(), userID(r), id)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	if !deleted {
		httpx.Error(w, 404, "Annotation not found")
		return
	}
	w.WriteHeader(204)
}
func (s Server) trending(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if len(category) < 2 || len(category) > 64 {
		if category != "" {
			httpx.Error(w, 400, "Invalid category")
			return
		}
		category = "cs.AI"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		httpx.Error(w, 400, "Invalid limit")
		return
	}
	items, cached, e := (services.Feed{Redis: s.Redis}).Trending(r.Context(), category, limit)
	if e != nil {
		httpx.Error(w, 502, e.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": items, "category": category, "cached": cached})
}
func orString(p *string, f string) string {
	if p == nil || *p == "" {
		return f
	}
	return *p
}

var _ = time.Second
