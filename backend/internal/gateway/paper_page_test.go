package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/google/uuid"
)

func TestPaperPageContainsMetadataAndApplicationForEveryVisitor(t *testing.T) {
	id := uuid.New()
	year := 2026
	frontend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index.html" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("application shell request must not forward visitor credentials or reader path")
		}
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Security-Policy", "script-src 'self'")
		_, _ = w.Write([]byte(`<html><head><title>Odyssey</title></head><body><div id="root"></div><script src="/assets/app.js"></script></body></html>`))
	}))
	defer frontend.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/papers/"+id.String() || r.Header.Get(identity.UserIDHeader) != "" ||
			r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("preview lookup must use anonymous public paper access")
		}
		_ = json.NewEncoder(w).Encode(catalog.PaperOut{
			ID: id, Title: `A <paper> & "results"`, Year: &year,
			Authors: []catalog.AuthorOut{{Name: "Author One"}},
		})
	}))
	defer upstream.Close()
	cfg := config.Config{
		FrontendServiceURL: frontend.URL, FrontendURL: "https://odyssey.example", CatalogURL: upstream.URL,
		IdentityURL: upstream.URL, LibraryURL: upstream.URL, AnnotationsURL: upstream.URL,
		AssistantURL: upstream.URL, FeedURL: upstream.URL, SearchAPIURL: upstream.URL,
	}
	handler := Handler(cfg)
	for _, agent := range []string{"TelegramBot (like TwitterBot)", "Mozilla/5.0"} {
		request := httptest.NewRequest(http.MethodGet, "/reader/"+id.String()+"?utm_source=telegram", nil)
		request.Header.Set("User-Agent", agent)
		request.Header.Set("Cookie", "private-session=value")
		request.Header.Set("Authorization", "Bearer an-expired-token")
		request.Header.Set(identity.UserIDHeader, uuid.NewString())
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		body := response.Body.String()
		for _, expected := range []string{
			`<meta property="og:title" content="A &lt;paper&gt; &amp; &#34;results&#34;">`,
			`<meta property="og:url" content="https://odyssey.example/reader/` + id.String() + `">`,
			`<meta property="og:image" content="https://odyssey.example/og-image.png">`,
			`Author One · 2026`, `<div id="root"></div>`, `/assets/app.js`,
		} {
			if !strings.Contains(body, expected) {
				t.Errorf("%s: missing %q in %s", agent, expected, body)
			}
		}
		if response.Code != 200 || response.Header().Get("Location") != "" ||
			response.Header().Get("Content-Security-Policy") != "script-src 'self'" || strings.Count(body, "<title>") != 1 {
			t.Fatalf("bad paper response: %d %v", response.Code, response.Header())
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/reader/"+id.String(), nil))
	if response.Code != 200 || response.Body.Len() != 0 || response.Header().Get("Content-Length") == "" {
		t.Fatalf("bad preview HEAD response: %d %v", response.Code, response.Header())
	}
}

func TestPaperPageDoesNotExposePrivateMetadataOrDependOnCatalogAvailability(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/index.html" {
					w.Header().Set("Content-Type", "text/html")
					_, _ = w.Write([]byte(`<html><head><title>Odyssey</title></head><body><div id="root"></div></body></html>`))
					return
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"title":"Confidential upload"}`))
			}))
			defer upstream.Close()
			handler := paperPageHandler{frontendURL: upstream.URL, catalogURL: upstream.URL, client: upstream.Client()}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/reader/"+uuid.NewString(), nil))
			if response.Code != 200 || strings.Contains(response.Body.String(), "Confidential") ||
				!strings.Contains(response.Body.String(), `<div id="root">`) {
				t.Fatalf("private or unavailable metadata broke the app shell: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestPaperPageRejectsInvalidPathsAndUnsupportedMethods(t *testing.T) {
	handler := paperPageHandler{}
	for _, test := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/reader/not-an-id", http.StatusNotFound},
		{http.MethodGet, "/reader/" + uuid.NewString() + "/extra", http.StatusNotFound},
		{http.MethodPost, "/reader/" + uuid.NewString(), http.StatusMethodNotAllowed},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status {
			t.Fatalf("%s %s returned %d", test.method, test.path, response.Code)
		}
	}
}
