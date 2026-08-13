package searchapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	DB       *pgxpool.Pool
	Provider Client
}

func (a API) Mount(r chi.Router) {
	r.Get("/search/chats", a.listChats)
	r.Get("/search/chats/{chatID}", a.getChat)
	r.Delete("/search/chats/{chatID}", a.deleteChat)
	r.Post("/search/chats/{chatID}/messages", a.sendMessage)
}

func (a API) store() Store { return Store{DB: a.DB} }

func (a API) listChats(w http.ResponseWriter, r *http.Request) {
	chats, err := a.store().List(r.Context(), identity.UserID(r))
	if err != nil {
		httpx.Error(w, 500, "Failed to load search chats")
		return
	}
	httpx.JSON(w, 200, chats)
}

func (a API) getChat(w http.ResponseWriter, r *http.Request) {
	chatID, err := uuid.Parse(chi.URLParam(r, "chatID"))
	if err != nil {
		httpx.Error(w, 404, "Search chat not found")
		return
	}
	chat, err := a.store().Get(r.Context(), identity.UserID(r), chatID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, 404, "Search chat not found")
		return
	}
	if err != nil {
		httpx.Error(w, 500, "Failed to load search chat")
		return
	}
	httpx.JSON(w, 200, chat)
}

func (a API) deleteChat(w http.ResponseWriter, r *http.Request) {
	chatID, err := uuid.Parse(chi.URLParam(r, "chatID"))
	if err != nil {
		httpx.Error(w, 404, "Search chat not found")
		return
	}
	deleted, err := a.store().Delete(r.Context(), identity.UserID(r), chatID)
	if err != nil {
		httpx.Error(w, 500, "Failed to delete search chat")
		return
	}
	if !deleted {
		httpx.Error(w, 404, "Search chat not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a API) sendMessage(w http.ResponseWriter, r *http.Request) {
	chatID, err := uuid.Parse(chi.URLParam(r, "chatID"))
	if err != nil {
		httpx.Error(w, 404, "Search chat not found")
		return
	}
	var body struct {
		Message string `json:"message"`
		Mode    string `json:"mode"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	body.Message = strings.TrimSpace(body.Message)
	if body.Message == "" {
		httpx.Error(w, 400, "Message is required")
		return
	}
	if len([]rune(body.Message)) > 20_000 {
		httpx.Error(w, 400, "Message is too long")
		return
	}
	if body.Mode != "deep" {
		body.Mode = "web"
	}
	store := a.store()
	title := truncateRunes(strings.Join(strings.Fields(body.Message), " "), 120)
	if err := store.Ensure(r.Context(), identity.UserID(r), chatID, title, body.Mode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, 404, "Search chat not found")
		} else {
			httpx.Error(w, 500, "Failed to create search chat")
		}
		return
	}
	if _, err := store.AddMessage(r.Context(), chatID, "user", body.Message, nil); err != nil {
		httpx.Error(w, 500, "Failed to save message")
		return
	}
	history, err := store.Messages(r.Context(), chatID, 40)
	if err != nil {
		httpx.Error(w, 500, "Failed to load chat context")
		return
	}
	providerMessages := make([]ProviderMessage, 0, len(history))
	for _, message := range history {
		providerMessages = append(providerMessages, ProviderMessage{Role: message.Role, Content: message.Content})
	}
	stream, err := a.Provider.Stream(r.Context(), providerMessages, body.Mode)
	if err != nil {
		httpx.Error(w, 502, err.Error())
		return
	}
	defer stream.Close()
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.Error(w, 500, "Streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	writeSSE(w, "meta", map[string]string{"chat_id": chatID.String()})
	flusher.Flush()

	var answer strings.Builder
	sources := make([]Source, 0)
	completed := false
	providerFailed := false
	err = stream.Events(r.Context(), func(event StreamEvent) error {
		switch event.Name {
		case "delta":
			var payload struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return err
			}
			answer.WriteString(payload.Content)
			if err := writeSSE(w, "delta", payload); err != nil {
				return err
			}
			flusher.Flush()
		case "progress":
			if _, err := fmt.Fprintf(w, "event: progress\ndata: %s\n\n", event.Data); err != nil {
				return err
			}
			flusher.Flush()
		case "source_progress":
			if _, err := fmt.Fprintf(w, "event: source_progress\ndata: %s\n\n", event.Data); err != nil {
				return err
			}
			flusher.Flush()
		case "sources":
			var payload struct {
				Sources []Source `json:"sources"`
			}
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return err
			}
			sources = payload.Sources
			if err := writeSSE(w, "sources", payload); err != nil {
				return err
			}
			flusher.Flush()
		case "error":
			completed = true
			providerFailed = true
			if _, err := fmt.Fprintf(w, "event: error\ndata: %s\n\n", event.Data); err != nil {
				return err
			}
			flusher.Flush()
		case "done":
			completed = true
		}
		return nil
	})
	if err != nil {
		_ = writeSSE(w, "error", map[string]string{"detail": err.Error()})
		flusher.Flush()
		return
	}
	if providerFailed {
		return
	}
	if !completed || strings.TrimSpace(answer.String()) == "" {
		_ = writeSSE(w, "error", map[string]string{"detail": "Web search stream ended before completion"})
		flusher.Flush()
		return
	}
	message, err := store.AddMessage(r.Context(), chatID, "assistant", answer.String(), sources)
	if err != nil {
		_ = writeSSE(w, "error", map[string]string{"detail": "Answer received but could not be saved"})
		flusher.Flush()
		return
	}
	_ = writeSSE(w, "done", message)
	flusher.Flush()
}

func writeSSE(w http.ResponseWriter, event string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	return err
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}
