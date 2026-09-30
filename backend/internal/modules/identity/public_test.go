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

// TestIsPublicRequestWhitelist pins the exact set of routes a guest may call.
// Every public route is listed here on purpose: widening the whitelist in
// IsPublicRequest must be a conscious change that also updates this table.
// The closed cases are the nearest neighbours of each public route, so an
// accidental prefix or method loosening is caught too.
func TestIsPublicRequestWhitelist(t *testing.T) {
	const id = "7a6d0d82-a527-4ab2-abd3-7d16f925ce13"
	cases := []struct {
		method, path string
		public       bool
	}{
		// The whole whitelist.
		{"GET", "/feed/trending", true},
		{"POST", "/papers/arxiv/open", true},
		{"GET", "/papers/" + id, true},
		{"GET", "/papers/" + id + "/pdf", true},
		{"GET", "/papers/" + id + "/pdf-url", true},
		{"POST", "/papers/" + id + "/translate", true},

		// Same paths, wrong method.
		{"POST", "/feed/trending", false},
		{"GET", "/papers/arxiv/open", false},
		{"POST", "/papers/" + id, false},
		{"DELETE", "/papers/" + id, false},
		{"POST", "/papers/" + id + "/pdf", false},
		{"GET", "/papers/" + id + "/translate", false},

		// Near misses in the path.
		{"GET", "/feed/trending/", false},
		{"GET", "/feed", false},
		{"POST", "/papers/arxiv", false},
		{"POST", "/papers/doi", false},
		{"POST", "/papers/upload", false},
		{"GET", "/papers/not-a-uuid", false},
		{"GET", "/papers/not-a-uuid/pdf", false},
		{"GET", "/papers/" + id + "/pdf/extra", false},
		{"POST", "/papers/" + id + "/translate/stream", false},

		// Personal paper routes.
		{"GET", "/papers/" + id + "/annotations", false},
		{"POST", "/papers/" + id + "/annotations", false},
		{"GET", "/papers/" + id + "/chat/messages", false},
		{"POST", "/papers/" + id + "/chat", false},
		{"POST", "/papers/" + id + "/explain", false},
		{"GET", "/papers/" + id + "/summary", false},
		{"POST", "/papers/" + id + "/summary", false},
		{"POST", "/papers/" + id + "/retry-pdf", false},
		{"POST", "/papers/" + id + "/find-fulltext", false},

		// Everything outside /papers and /feed/trending.
		{"GET", "/auth/me", false},
		{"GET", "/library", false},
		{"GET", "/library/folders", false},
		{"POST", "/library/" + id, false},
		{"GET", "/search/chats", false},
		{"GET", "/assistant/models", false},
		{"GET", "/", false},
	}
	for _, c := range cases {
		got := IsPublicRequest(httptest.NewRequest(c.method, c.path, nil))
		if got != c.public {
			t.Errorf("%s %s: public=%v, want %v", c.method, c.path, got, c.public)
		}
	}
}
