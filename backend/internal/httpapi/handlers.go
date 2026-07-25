package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"assistant/internal/domain"
	"assistant/internal/qa"
)

// QAService is the orchestration API the handlers depend on.
type QAService interface {
	Prepare(ctx context.Context, req qa.Request) (*qa.Prepared, error)
	Stream(ctx context.Context, p *qa.Prepared, onDelta func(string) error) (qa.Result, error)
	History(ctx context.Context, chatID string) ([]domain.Message, error)
}

type handlers struct {
	svc    QAService
	logger *slog.Logger
}

// getMessages returns the stored history for a chat.
func (h *handlers) getMessages(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatId")

	msgs, err := h.svc.History(r.Context(), chatID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

type postMessageRequest struct {
	ArticleID string `json:"articleId"`
	Content   string `json:"content"`
}

// postMessage accepts a question and streams the LLM answer as SSE.
func (h *handlers) postMessage(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatId")

	var body postMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid JSON body"})
		return
	}

	req := qa.Request{ChatID: chatID, ArticleID: body.ArticleID, Content: body.Content}

	// Validate and gather context first, while we can still return a plain HTTP
	// status (400 for bad input, 502 for upstream failures).
	prepared, err := h.svc.Prepare(r.Context(), req)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	// From here the response is committed to SSE; later failures are reported as
	// SSE error events, not HTTP statuses.
	sse, ok := newSSEWriter(w)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "streaming unsupported"})
		return
	}

	onDelta := func(text string) error {
		return sse.event("delta", deltaEvent{Text: text})
	}

	result, err := h.svc.Stream(r.Context(), prepared, onDelta)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "stream failed", "error", err, "chatId", chatID)
		_ = sse.event("error", errorBody{Error: publicError(err)})
		return
	}

	_ = sse.event("done", doneEvent{MessageID: result.MessageID, Content: result.Answer})
}

type deltaEvent struct {
	Text string `json:"text"`
}

type doneEvent struct {
	MessageID string `json:"messageId"`
	Content   string `json:"content"`
}

type errorBody struct {
	Error string `json:"error"`
}

func (h *handlers) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, qa.ErrInvalidRequest):
		status = http.StatusBadRequest
	case errors.Is(err, qa.ErrUpstream):
		status = http.StatusBadGateway
	}
	h.logger.ErrorContext(r.Context(), "request failed", "error", err, "status", status)
	writeJSON(w, status, errorBody{Error: publicError(err)})
}

// publicError avoids leaking upstream internals while keeping the category.
func publicError(err error) string {
	switch {
	case errors.Is(err, qa.ErrInvalidRequest):
		return err.Error()
	case errors.Is(err, qa.ErrUpstream):
		return "upstream service unavailable"
	case errors.Is(err, qa.ErrLLM):
		return "language model error"
	case errors.Is(err, qa.ErrPersist):
		return "failed to save conversation"
	default:
		return "internal error"
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
