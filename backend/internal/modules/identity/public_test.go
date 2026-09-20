package identity

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestGuestMiddleware(t *testing.T) {
	for name, middleware := range map[string]func(http.Handler) http.Handler{
		"monolith": (API{}).GuestOrAuthenticated,
		"service":  GuestOrAuthenticatedFromGateway,
	} {
		t.Run(name, func(t *testing.T) {
			h := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if UserID(r) != uuid.Nil {
					t.Fatal("expected a guest context")
				}
				w.WriteHeader(204)
			}))
			for _, path := range []string{"/feed/trending", "/papers/7a6d0d82-a527-4ab2-abd3-7d16f925ce13/pdf", "/library", "/search/chats"} {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
				want := 204
				if path == "/library" || path == "/search/chats" {
					want = 401
				}
				if w.Code != want {
					t.Fatalf("%s: status=%d, want %d", path, w.Code, want)
				}
			}
		})
	}
}
