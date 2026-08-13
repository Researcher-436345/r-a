package translation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
		httpx.JSON(w, http.StatusOK, map[string]any{"status": status, "max_chars": cfg.TranslationMaxChars, "model": cfg.TranslationLLMModel})
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
		if wantsStream(r) {
			serveStream(w, r, service, cfg, req)
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

func wantsStream(r *http.Request) bool {
	return r.URL.Query().Get("stream") == "1" || strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

func serveStream(w http.ResponseWriter, r *http.Request, service Service, cfg config.Config, input Request) {
	validated, err := Validate(input, cfg.TranslationMaxChars)
	if err != nil {
		var validationErr *ValidationError
		if errors.As(err, &validationErr) {
			status := http.StatusBadRequest
			if len([]rune(input.Text)) > cfg.TranslationMaxChars {
				status = http.StatusRequestEntityTooLarge
			}
			httpx.Error(w, status, validationErr.Detail)
			return
		}
	}
	if strings.TrimSpace(cfg.LLMAPIKey) == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "Translation provider is not configured")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.Error(w, http.StatusInternalServerError, "Streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	write := func(event StreamEvent) error {
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	result, err := service.TranslateStream(r.Context(), validated, func(delta string) error {
		return write(StreamEvent{Type: "delta", Text: delta})
	})
	if err != nil {
		detail := "Translation provider is unavailable"
		if errors.Is(err, ErrNotConfigured) {
			detail = "Translation provider is not configured"
		}
		_ = write(StreamEvent{Type: "error", Detail: detail})
		return
	}
	_ = write(StreamEvent{Type: "done", Translation: result.Translation, TargetLang: result.TargetLang})
}
