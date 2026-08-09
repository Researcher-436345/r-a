package translation

import (
	"errors"
	"net/http"

	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

func Handler(cfg config.Config) http.Handler {
	service := Service{Config: cfg}
	sem := make(chan struct{}, cfg.TranslationMaxConcurrent)
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		status := "ok"
		if cfg.LLMAPIKey == "" {
			status = "not_configured"
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"status": status, "max_chars": cfg.TranslationMaxChars})
	})
	r.Post("/translate", func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			httpx.Error(w, http.StatusTooManyRequests, "Translation service is busy")
			return
		}
		var req Request
		if !httpx.DecodeJSON(w, r, &req) {
			return
		}
		result, err := service.Translate(r.Context(), req)
		if err == nil {
			httpx.JSON(w, http.StatusOK, result)
			return
		}
		var validationErr *ValidationError
		switch {
		case errors.As(err, &validationErr):
			status := http.StatusBadRequest
			if len([]rune(req.Text)) > cfg.TranslationMaxChars {
				status = http.StatusRequestEntityTooLarge
			}
			httpx.Error(w, status, validationErr.Detail)
		case errors.Is(err, ErrNotConfigured):
			httpx.Error(w, http.StatusServiceUnavailable, "Translation provider is not configured")
		default:
			httpx.Error(w, http.StatusBadGateway, "Translation provider is unavailable")
		}
	})
	return r
}
