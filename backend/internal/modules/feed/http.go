package feed

import (
	"net/http"
	"strconv"

	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

type API struct {
	Service Service
}

func (a API) Mount(r chi.Router) {
	r.Get("/feed/trending", a.trending)
}

func (a API) trending(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if len(category) < 2 || len(category) > 64 {
		if category != "" {
			httpx.Error(w, 400, "Invalid category")
			return
		}
		category = "cs.AI"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		httpx.Error(w, 400, "Invalid limit")
		return
	}

	mode := ParseSortMode(r.URL.Query().Get("sort"))
	if mode == "" && r.URL.Query().Get("sort") != "" {
		httpx.Error(w, 400, "Invalid sort (use new, hot, popular)")
		return
	}
	if mode == "" {
		mode = SortNew
	}

	items, cached, e := a.Service.Trending(r.Context(), category, limit, mode)
	if e != nil {
		httpx.Error(w, 502, e.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{
		"items":    items,
		"category": category,
		"sort":     mode,
		"cached":   cached,
	})
}
