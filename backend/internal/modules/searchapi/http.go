package searchapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/observability"
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
	run, err := a.recoverResearch(r.Context(), identity.UserID(r), chatID)
	if err != nil {
		httpx.Error(w, 502, "Failed to recover research")
		return
	}
	if run != nil && run.Status == "completed" {
		chat, err = a.store().Get(r.Context(), identity.UserID(r), chatID)
		if err != nil {
			httpx.Error(w, 500, "Failed to load completed research")
			return
		}
	}
	chat.Research = run
	httpx.JSON(w, 200, chat)
}

func (a API) deleteChat(w http.ResponseWriter, r *http.Request) {
	chatID, err := uuid.Parse(chi.URLParam(r, "chatID"))
	if err != nil {
		httpx.Error(w, 404, "Search chat not found")
		return
	}
	run, err := a.store().LatestRun(r.Context(), identity.UserID(r), chatID)
	if err != nil {
		httpx.Error(w, 500, "Failed to load research")
		return
	}
	if run != nil && run.Status == "running" && run.ProviderResponseID != nil {
		if err := a.Provider.Cancel(r.Context(), *run.ProviderResponseID); err != nil {
			httpx.Error(w, 502, "Failed to cancel research")
			return
		}
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
	if body.Mode == "deep" {
		observability.SetFeature(r.Context(), "deep_research")
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
	ctx := r.Context()
	if body.Mode == "deep" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), a.Provider.DeepTimeout)
		defer cancel()
	}
	runID := uuid.Nil
	runFinished := false
	failure := "Исследование прервано до завершения"
	defer func() {
		if runID != uuid.Nil && !runFinished {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = store.FailRun(cleanup, runID, failure)
		}
	}()

	if body.Mode == "deep" {
		runID, err = store.StartRun(ctx, chatID, body.Message)
	} else {
		_, err = store.AddMessage(ctx, chatID, "user", body.Message, nil)
	}
	if err != nil {
		if errors.Is(err, ErrResearchRunning) {
			httpx.Error(w, 409, "Research is already running in this chat")
		} else {
			httpx.Error(w, 500, "Failed to save message")
		}
		return
	}

	history, err := store.Messages(ctx, chatID, 40)
	if err != nil {
		httpx.Error(w, 500, "Failed to load chat context")
		return
	}
	providerMessages := make([]ProviderMessage, 0, len(history))
	for _, message := range history {
		providerMessages = append(providerMessages, ProviderMessage{Role: message.Role, Content: message.Content})
	}
	stream, err := a.Provider.Stream(ctx, providerMessages, body.Mode)
	if err != nil {
		failure = err.Error()
		httpx.Error(w, 502, failure)
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
	_ = writeSSE(w, "meta", map[string]string{"chat_id": chatID.String()})
	flusher.Flush()

	connected := true
	emit := func(event StreamEvent) error {
		if !connected || (body.Mode == "deep" && r.Context().Err() != nil) {
			return nil
		}
		_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Name, event.Data)
		if err != nil && body.Mode == "deep" {
			connected = false
			return nil
		}
		flusher.Flush()
		return err
	}
	saveResponseID := func(id string) error {
		if runID == uuid.Nil {
			return nil
		}
		return store.SaveResponseID(ctx, runID, id)
	}
	answer, sources, err := consumeSearchStream(ctx, stream, emit, saveResponseID)
	send := func(name string, payload any) {
		if connected && r.Context().Err() == nil {
			_ = writeSSE(w, name, payload)
			flusher.Flush()
		}
	}
	if err != nil {
		failure = err.Error()
		send("error", map[string]string{"detail": failure})
		return
	}
	var message Message
	if runID != uuid.Nil {
		message, err = store.CompleteRun(ctx, runID, answer, sources)
	} else {
		message, err = store.AddMessage(ctx, chatID, "assistant", answer, sources)
	}
	if err != nil {
		failure = "Answer received but could not be saved"
		send("error", map[string]string{"detail": failure})
		return
	}
	runFinished = true
	send("done", message)
}

func (a API) recoverResearch(ctx context.Context, userID, chatID uuid.UUID) (*ResearchRun, error) {
	store := a.store()
	run, err := store.LatestRun(ctx, userID, chatID)
	if err != nil || run == nil || run.Status != "running" {
		return run, err
	}
	if run.ProviderResponseID == nil {
		if time.Since(run.CreatedAt) > a.Provider.DeepTimeout {
			detail := "Исследование прервано до получения идентификатора OpenAI"
			if err := store.FailRun(ctx, run.ID, detail); err != nil {
				return nil, err
			}
			run.Status, run.ErrorMessage = "failed", &detail
		}
		return run, nil
	}
	result, err := a.Provider.Result(ctx, *run.ProviderResponseID)
	if err != nil {
		return nil, err
	}
	switch result.Status {
	case "completed":
		if _, err := store.CompleteRun(ctx, run.ID, result.Content, result.Sources); err != nil {
			return nil, err
		}
		run.Status = "completed"
	case "failed":
		if err := store.FailRun(ctx, run.ID, result.Detail); err != nil {
			return nil, err
		}
		run.Status, run.ErrorMessage = "failed", &result.Detail
	case "queued", "in_progress":
		if time.Since(run.CreatedAt) > a.Provider.DeepTimeout {
			if err := a.Provider.Cancel(ctx, *run.ProviderResponseID); err != nil {
				return nil, err
			}
			detail := "Истёк лимит времени глубокого исследования"
			if err := store.FailRun(ctx, run.ID, detail); err != nil {
				return nil, err
			}
			run.Status, run.ErrorMessage = "failed", &detail
		}
	}
	return run, nil
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
