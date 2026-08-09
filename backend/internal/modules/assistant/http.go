package assistant

import (
	"errors"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/modules/translation"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	r.Post("/papers/{paperID}/chat", a.chat)
	r.Post("/papers/{paperID}/explain", a.explain)
	r.Post("/papers/{paperID}/translate", a.translate)
}

func (a API) papers() catalog.Store { return catalog.Store{DB: a.DB} }
func (a API) llm() LLM              { return LLM{Config: a.Config} }

func (a API) chat(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
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
	p, e := a.papers().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 404, "Paper not found")
		return
	}
	httpx.JSON(w, 200, map[string]string{"reply": a.llm().Chat(r.Context(), p, b.Message, b.ContextText)})
}

func (a API) explain(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
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
	p, e := a.papers().GetPaperOut(r.Context(), id)
	if e != nil {
		httpx.Error(w, 404, "Paper not found")
		return
	}
	httpx.JSON(w, 200, map[string]string{"reply": a.llm().Explain(r.Context(), p, b.Text, b.Question)})
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
