package annotations

import (
	"errors"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PaperGate checks that the user can access a paper (library membership).
type PaperGate interface {
	RequirePaper(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool)
}

type API struct {
	Store Store
	Papers PaperGate
}

func (a API) Mount(r chi.Router) {
	r.Get("/papers/{paperID}/annotations", a.list)
	r.Post("/papers/{paperID}/annotations", a.create)
	r.Patch("/annotations/{annotationID}", a.patch)
	r.Delete("/annotations/{annotationID}", a.delete)
}

func (a API) list(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	out, e := a.Store.List(r.Context(), identity.UserID(r), id)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 200, out)
}

func (a API) create(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	var b struct {
		Page                int     `json:"page"`
		Rect                *Rect   `json:"rect"`
		SelectedText        string  `json:"selected_text"`
		Note                string  `json:"note"`
		Color               string  `json:"color"`
		SourceChatMessageID *string `json:"source_chat_message_id"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.Page < 0 || strings.TrimSpace(b.SelectedText) == "" {
		httpx.Error(w, 400, "Invalid annotation")
		return
	}
	if b.Color == "" {
		b.Color = "#facc15"
	}
	var sourceMsg *uuid.UUID
	if b.SourceChatMessageID != nil && strings.TrimSpace(*b.SourceChatMessageID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*b.SourceChatMessageID))
		if err != nil {
			httpx.Error(w, 400, "Invalid source_chat_message_id")
			return
		}
		sourceMsg = &parsed
	}
	ann, e := a.Store.Create(r.Context(), identity.UserID(r), id, b.Page, b.Rect, strings.TrimSpace(b.SelectedText), strings.TrimSpace(b.Note), b.Color, sourceMsg)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 201, ann)
}

func (a API) patch(w http.ResponseWriter, r *http.Request) {
	id, e := uuid.Parse(chi.URLParam(r, "annotationID"))
	if e != nil {
		httpx.Error(w, 404, "Not found")
		return
	}
	var b struct {
		Note string `json:"note"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	ann, e := a.Store.Patch(r.Context(), identity.UserID(r), id, strings.TrimSpace(b.Note))
	if errors.Is(e, pgx.ErrNoRows) {
		httpx.Error(w, 404, "Annotation not found")
		return
	}
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 200, ann)
}

func (a API) delete(w http.ResponseWriter, r *http.Request) {
	id, e := uuid.Parse(chi.URLParam(r, "annotationID"))
	if e != nil {
		httpx.Error(w, 404, "Not found")
		return
	}
	deleted, e := a.Store.Delete(r.Context(), identity.UserID(r), id)
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
