package library

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type API struct {
	Store Store
}

func (a API) Mount(r chi.Router) {
	r.Get("/library", a.list)
	r.Get("/library/folders", a.folders)
	r.Post("/library/folders", a.createFolder)
	r.Delete("/library/folders/{folderID}", a.deleteFolder)
	r.Get("/library/{paperID}", a.item)
	r.Post("/library/{paperID}", a.add)
	r.Patch("/library/{paperID}", a.patch)
	r.Delete("/library/{paperID}", a.delete)
}

func (a API) item(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "paperID"))
	if err != nil {
		httpx.Error(w, 404, "Library item not found")
		return
	}
	out, err := a.Store.Item(r.Context(), identity.UserID(r), id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, 404, "Library item not found")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, out)
}

func (a API) add(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "paperID"))
	if err != nil {
		httpx.Error(w, 404, "Paper not found")
		return
	}
	var body struct {
		FolderID *uuid.UUID `json:"folder_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	userID := identity.UserID(r)
	inLibrary, err := a.Store.Has(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if !inLibrary {
		public, publicErr := a.Store.Catalog.IsPublic(r.Context(), id)
		if publicErr != nil {
			httpx.Error(w, 500, publicErr.Error())
			return
		}
		if !public {
			httpx.Error(w, 404, "Paper not found")
			return
		}
		if err = a.Store.Add(r.Context(), userID, id); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
	}
	if body.FolderID != nil {
		out, patchErr := a.Store.Patch(r.Context(), userID, id, nil, nil, body.FolderID)
		if errors.Is(patchErr, pgx.ErrNoRows) {
			httpx.Error(w, 404, "Folder not found")
			return
		}
		if patchErr != nil {
			httpx.Error(w, 500, patchErr.Error())
			return
		}
		httpx.JSON(w, http.StatusCreated, out)
		return
	}
	out, err := a.Store.Item(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a API) list(w http.ResponseWriter, r *http.Request) {
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
	var folderID *uuid.UUID
	if raw := r.URL.Query().Get("folder_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, 400, "Invalid folder id")
			return
		}
		folderID = &parsed
	}
	items, total, e := a.Store.List(r.Context(), identity.UserID(r), page, limit, status, folderID)
	if e != nil {
		httpx.Error(w, 500, e.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": items, "page": page, "limit": limit, "total": total})
}

func (a API) patch(w http.ResponseWriter, r *http.Request) {
	id, e := uuid.Parse(chi.URLParam(r, "paperID"))
	if e != nil {
		httpx.Error(w, 404, "Not found")
		return
	}
	var b struct {
		Status   *string    `json:"status"`
		Favorite *bool      `json:"favorite"`
		FolderID *uuid.UUID `json:"folder_id"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.Status != nil && *b.Status != "unread" && *b.Status != "reading" && *b.Status != "read" {
		httpx.Error(w, 400, "Invalid status")
		return
	}
	out, e := a.Store.Patch(r.Context(), identity.UserID(r), id, b.Status, b.Favorite, b.FolderID)
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

func (a API) folders(w http.ResponseWriter, r *http.Request) {
	folders, err := a.Store.Folders(r.Context(), identity.UserID(r))
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": folders})
}

func (a API) createFolder(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name     string     `json:"name"`
		ParentID *uuid.UUID `json:"parent_id"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" || len([]rune(b.Name)) > 120 {
		httpx.Error(w, 400, "Folder name must contain 1 to 120 characters")
		return
	}
	folder, err := a.Store.CreateFolder(r.Context(), identity.UserID(r), b.Name, b.ParentID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, 404, "Parent folder not found")
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		httpx.Error(w, 409, "A folder with this name already exists here")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 201, folder)
}

func (a API) deleteFolder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "folderID"))
	if err != nil {
		httpx.Error(w, 404, "Folder not found")
		return
	}
	deleted, err := a.Store.DeleteFolder(r.Context(), identity.UserID(r), id)
	if err != nil {
		if errors.Is(err, ErrSystemFolderDeletion) {
			httpx.Error(w, 400, "System folders cannot be deleted")
			return
		}
		httpx.Error(w, 500, err.Error())
		return
	}
	if !deleted {
		httpx.Error(w, 404, "Folder not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a API) delete(w http.ResponseWriter, r *http.Request) {
	id, e := uuid.Parse(chi.URLParam(r, "paperID"))
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
		httpx.Error(w, 404, "Library item not found")
		return
	}
	w.WriteHeader(204)
}
