package assistant

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/modules/content"
	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/modules/translation"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaperGate interface {
	RequirePaper(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool)
}

type API struct {
	Config config.Config
	DB     *pgxpool.Pool
	Papers PaperGate
}

func (a API) Mount(r chi.Router) {
	r.Get("/assistant/models", a.listModels)
	r.Get("/papers/{paperID}/chat/messages", a.listMessages)
	r.Get("/papers/{paperID}/chat/context", a.chatContext)
	r.Post("/papers/{paperID}/chat", a.chat)
	r.Post("/papers/{paperID}/explain", a.explain)
	r.Post("/papers/{paperID}/translate", a.translate)
}

func (a API) papers() catalog.Store { return catalog.Store{DB: a.DB} }
func (a API) store() Store          { return Store{DB: a.DB} }
func (a API) docs() content.Store   { return content.Store{DB: a.DB} }

func (a API) llm(model string) LLM {
	return LLM{Config: a.Config, Model: model}
}

func (a API) resolveModel(requested string) (string, bool) {
	return a.Config.ResolveLLMModel(requested)
}

func (a API) listModels(w http.ResponseWriter, r *http.Request) {
	models := a.Config.LLMModels
	if models == nil {
		models = []config.LLMModelOption{}
	}
	httpx.JSON(w, 200, map[string]any{
		"default": a.Config.LLMModel,
		"items":   models,
	})
}

func (a API) listMessages(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	items, err := a.store().List(r.Context(), identity.UserID(r), id)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if items == nil {
		items = []Message{}
	}
	httpx.JSON(w, 200, map[string]any{"items": items})
}

func (a API) loadPaperContext(w http.ResponseWriter, r *http.Request, id uuid.UUID) (
	p catalog.PaperOut,
	fullPaper string,
	threadSummary string,
	history []Message,
	ok bool,
) {
	var e error
	p, e = a.papers().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 404, "Paper not found")
		return p, "", "", nil, false
	}
	userID := identity.UserID(r)
	history, e = a.store().List(r.Context(), userID, id)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return p, "", "", nil, false
	}
	if doc, err := a.docs().GetDocument(r.Context(), id); err == nil && doc.Status == "ready" {
		plain := strings.TrimSpace(doc.PlainText)
		if plain == "" {
			plain = strings.TrimSpace(doc.Markdown)
		}
		chunks, chunkErr := a.docs().ListChunks(r.Context(), id)
		if chunkErr != nil {
			httpx.Error(w, 500, chunkErr.Error())
			return p, "", "", nil, false
		}
		fullPaper = FormatPaperWithPageMarkers(chunks, plain)
	}
	if ts, err := a.docs().GetThreadSummary(r.Context(), userID, id); err == nil {
		threadSummary = ts.Summary
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, 500, err.Error())
		return p, "", "", nil, false
	}
	return p, fullPaper, threadSummary, history, true
}

func (a API) chatContext(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	modelID, okModel := a.resolveModel(r.URL.Query().Get("model"))
	if !okModel {
		httpx.Error(w, 400, "unknown model")
		return
	}
	p, fullPaper, threadSummary, history, ok := a.loadPaperContext(w, r, id)
	if !ok {
		return
	}
	usage := a.llm(modelID).EstimateChatContext(p, fullPaper, threadSummary, historyTurns(history))
	httpx.JSON(w, 200, usage)
}

func (a API) chat(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	var b struct {
		Message     string  `json:"message"`
		ContextText *string `json:"context_text"`
		Model       string  `json:"model"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if strings.TrimSpace(b.Message) == "" {
		httpx.Error(w, 400, "message is required")
		return
	}
	modelID, okModel := a.resolveModel(b.Model)
	if !okModel {
		httpx.Error(w, 400, "unknown model")
		return
	}

	p, fullPaper, threadSummary, history, ok := a.loadPaperContext(w, r, id)
	if !ok {
		return
	}

	userID := identity.UserID(r)
	contextText := ""
	if b.ContextText != nil {
		contextText = strings.TrimSpace(*b.ContextText)
	}
	var contextPtr *string
	if contextText != "" {
		contextPtr = &contextText
	}

	if wantsChatStream(r) {
		a.chatStream(w, r, id, userID, p, fullPaper, threadSummary, history, modelID, strings.TrimSpace(b.Message), contextText, contextPtr)
		return
	}

	keep := a.Config.ChatRecentKeep
	result, err := a.llm(modelID).ChatWithPaper(
		r.Context(),
		p,
		fullPaper,
		threadSummary,
		historyTurns(history),
		strings.TrimSpace(b.Message),
		contextText,
	)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrLLMNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		httpx.Error(w, status, err.Error())
		return
	}

	if strings.TrimSpace(result.Summary) != "" && result.Summary != threadSummary {
		_ = a.docs().UpsertThreadSummary(r.Context(), userID, id, result.Summary, LastCoveredID(history, keep))
	}

	userMsg, assistantMsg, err := a.store().AppendPair(r.Context(), userID, id, strings.TrimSpace(b.Message), contextPtr, result.Reply)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}

	httpx.JSON(w, 200, map[string]any{
		"reply":             result.Reply,
		"message_id":        assistantMsg.ID,
		"user_message_id":   userMsg.ID,
		"user_message":      userMsg,
		"assistant_message": assistantMsg,
		"context_usage":     result.Usage,
	})
}

func wantsChatStream(r *http.Request) bool {
	if r.URL.Query().Get("stream") == "1" {
		return true
	}
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

func (a API) chatStream(
	w http.ResponseWriter,
	r *http.Request,
	paperID uuid.UUID,
	userID uuid.UUID,
	p catalog.PaperOut,
	fullPaper, threadSummary string,
	history []Message,
	modelID, message, contextText string,
	contextPtr *string,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.Error(w, 500, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	writeEvent := func(payload map[string]any) bool {
		raw, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	keep := a.Config.ChatRecentKeep
	result, err := a.llm(modelID).ChatWithPaperStream(
		r.Context(),
		p,
		fullPaper,
		threadSummary,
		historyTurns(history),
		message,
		contextText,
		func(delta string) error {
			if !writeEvent(map[string]any{"type": "delta", "text": delta}) {
				return errors.New("client disconnected")
			}
			return nil
		},
	)
	if err != nil {
		_ = writeEvent(map[string]any{"type": "error", "detail": err.Error()})
		return
	}

	if strings.TrimSpace(result.Summary) != "" && result.Summary != threadSummary {
		_ = a.docs().UpsertThreadSummary(r.Context(), userID, paperID, result.Summary, LastCoveredID(history, keep))
	}

	userMsg, assistantMsg, err := a.store().AppendPair(r.Context(), userID, paperID, message, contextPtr, result.Reply)
	if err != nil {
		_ = writeEvent(map[string]any{"type": "error", "detail": err.Error()})
		return
	}

	_ = writeEvent(map[string]any{
		"type":              "done",
		"reply":             result.Reply,
		"message_id":        assistantMsg.ID,
		"user_message_id":   userMsg.ID,
		"user_message":      userMsg,
		"assistant_message": assistantMsg,
		"context_usage":     result.Usage,
	})
}

func (a API) explain(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	var b struct {
		Text     string `json:"text"`
		Question string `json:"question"`
		Model    string `json:"model"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if strings.TrimSpace(b.Text) == "" {
		httpx.Error(w, 400, "text is required")
		return
	}
	modelID, okModel := a.resolveModel(b.Model)
	if !okModel {
		httpx.Error(w, 400, "unknown model")
		return
	}
	p, e := a.papers().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 404, "Paper not found")
		return
	}
	reply, err := a.llm(modelID).Explain(r.Context(), p, strings.TrimSpace(b.Text), strings.TrimSpace(b.Question))
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrLLMNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		httpx.Error(w, status, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]string{"reply": reply})
}

func (a API) translate(w http.ResponseWriter, r *http.Request) {
	_, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	var b struct {
		Text       string `json:"text"`
		TargetLang string `json:"target_lang"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.TargetLang == "" {
		b.TargetLang = "ru"
	}
	input, err := translation.Validate(translation.Request{Text: b.Text, TargetLang: b.TargetLang}, a.Config.TranslationMaxChars)
	if err != nil {
		status := http.StatusBadRequest
		if len([]rune(strings.TrimSpace(b.Text))) > a.Config.TranslationMaxChars {
			status = http.StatusRequestEntityTooLarge
		}
		httpx.Error(w, status, err.Error())
		return
	}
	result, err := (TranslationClient{Config: a.Config}).Translate(r.Context(), input)
	if err != nil {
		var serviceErr *TranslationServiceError
		if errors.As(err, &serviceErr) {
			httpx.Error(w, serviceErr.Status, serviceErr.Detail)
			return
		}
		httpx.Error(w, http.StatusBadGateway, "Translation service is unavailable")
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
