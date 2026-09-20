package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/google/uuid"
)

func TestGuestAccessBoundary(t *testing.T) {
	const paper = "/papers/7a6d0d82-a527-4ab2-abd3-7d16f925ce13"
	var forwardedUser string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedUser = r.Header.Get(identity.UserIDHeader)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	cfg := config.Config{
		JWTSecret: "test-secret", IdentityURL: upstream.URL, CatalogURL: upstream.URL,
		LibraryURL: upstream.URL, AnnotationsURL: upstream.URL, AssistantURL: upstream.URL,
		FeedURL: upstream.URL, SearchAPIURL: upstream.URL,
	}
	h := Handler(cfg)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/feed/trending?sort=hot", 204},
		{"POST", "/papers/arxiv/open", 204},
		{"GET", paper, 204},
		{"GET", paper + "/pdf", 204},
		{"GET", paper + "/pdf-url", 204},
		{"POST", paper + "/translate?stream=1", 204},
		{"GET", "/library", 401},
		{"POST", "/library/123", 401},
		{"DELETE", "/library/123", 401},
		{"GET", "/search/chats", 401},
		{"POST", "/search/chats/123/messages", 401},
		{"POST", "/papers/arxiv", 401},
		{"POST", "/papers/doi", 401},
		{"POST", "/papers/from-url", 401},
		{"POST", "/papers/upload", 401},
		{"GET", paper + "/annotations", 401},
		{"POST", paper + "/annotations", 401},
		{"GET", paper + "/chat/messages", 401},
		{"GET", paper + "/chat/context", 401},
		{"POST", paper + "/chat?stream=1", 401},
		{"POST", paper + "/explain", 401},
		{"POST", paper + "/retry-pdf", 401},
		{"DELETE", paper + "/pdf", 401},
		{"POST", paper + "/translate/extra", 401},
		{"GET", "/assistant/models", 401},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			forwardedUser = "not-called"
			r := httptest.NewRequest(tc.method, tc.path, nil)
			// Spoofing a gateway header must never authenticate a guest.
			r.Header.Set(identity.UserIDHeader, uuid.NewString())
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == 204 && forwardedUser != "" {
				t.Fatalf("guest forwarded with user identity %q", forwardedUser)
			}
			if tc.status == 401 && forwardedUser != "not-called" {
				t.Fatal("protected request reached upstream")
			}
		})
	}
	user := uuid.New()
	token, err := identity.IssueAccessToken(cfg.JWTSecret, user, uuid.New(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/library", paper, "/search/chats"} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set(identity.UserIDHeader, uuid.NewString())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 204 || forwardedUser != user.String() {
			t.Fatalf("authenticated request %s: status=%d user=%q", path, w.Code, forwardedUser)
		}
	}
}
