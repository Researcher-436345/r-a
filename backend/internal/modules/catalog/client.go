package catalog

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MountInternal registers service-to-service routes (not exposed via gateway publicly).
func (a API) MountInternal(r chi.Router) {
	r.Get("/internal/papers/{paperID}/access", a.internalAccess)
}

func (a API) internalAccess(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "paperID")
	if !ok {
		return
	}
	userID, err := uuid.Parse(strings.TrimSpace(r.Header.Get(identity.UserIDHeader)))
	if err != nil {
		httpx.Error(w, 401, "Not authenticated")
		return
	}
	allowed, e := a.canAccessPaper(r.Context(), userID, id)
	if e != nil || !allowed {
		httpx.Error(w, 404, "Paper not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PaperClient checks paper access via catalog internal API.
type PaperClient struct {
	BaseURL string
	HTTP    *http.Client
	Token   string
}

func (c PaperClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// RequirePaper implements assistant/annotations PaperGate over HTTP.
func (c PaperClient) RequirePaper(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "paperID"))
	if err != nil {
		httpx.Error(w, 404, "Not found")
		return uuid.Nil, false
	}
	base := strings.TrimRight(c.BaseURL, "/")
	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		fmt.Sprintf("%s/internal/papers/%s/access", base, id),
		nil,
	)
	if err != nil {
		httpx.Error(w, 502, "catalog unavailable")
		return id, false
	}
	req.Header.Set(identity.UserIDHeader, identity.UserID(r).String())
	if c.Token != "" {
		req.Header.Set("X-Internal-Token", c.Token)
	}
	resp, err := c.client().Do(req)
	if err != nil {
		httpx.Error(w, 502, "catalog unavailable")
		return id, false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return id, true
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if len(body) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
		return id, false
	}
	httpx.Error(w, resp.StatusCode, "Paper not found")
	return id, false
}
