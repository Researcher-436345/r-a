package identity

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// IsPublicRequest is shared by the gateway, domain services and monolith.
// Public paper handlers still enforce access to public documents only.
func IsPublicRequest(r *http.Request) bool {
	path := r.URL.Path
	if r.Method == http.MethodGet && path == "/feed/trending" {
		return true
	}
	if r.Method == http.MethodPost && path == "/papers/arxiv/open" {
		return true
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "papers" {
		return false
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return false
	}
	if r.Method == http.MethodGet {
		return len(parts) == 2 || (len(parts) == 3 && (parts[2] == "pdf" || parts[2] == "pdf-url"))
	}
	return r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "translate"
}

// GuestOrAuthenticated retains strict authentication on every other endpoint.
func (a API) GuestOrAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsPublicRequest(r) && r.Header.Get("Authorization") == "" {
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), uuid.Nil)))
			return
		}
		a.Middleware(next).ServeHTTP(w, r)
	})
}

func GuestOrAuthenticatedFromGateway(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsPublicRequest(r) && r.Header.Get(UserIDHeader) == "" {
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), uuid.Nil)))
			return
		}
		MiddlewareFromGateway(next).ServeHTTP(w, r)
	})
}
