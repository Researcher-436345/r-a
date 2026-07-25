// Package httpapi wires HTTP routes, middleware and SSE streaming for the
// assistant service.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// NewRouter builds the HTTP handler with routes and middleware applied.
// requestTimeout caps the lifetime of each request's context (including the
// streaming endpoint), so it must be generous enough for long LLM answers.
func NewRouter(svc QAService, logger *slog.Logger, requestTimeout time.Duration) http.Handler {
	h := &handlers{svc: svc, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /v1/chats/{chatId}/messages", h.getMessages)
	mux.HandleFunc("POST /v1/chats/{chatId}/messages", h.postMessage)

	return chain(mux, corsMiddleware, contextTimeout(requestTimeout), requestLogger(logger))
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
