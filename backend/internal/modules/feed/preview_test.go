package feed

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type previewTransport func(*http.Request) (*http.Response, error)

func (f previewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPreviewRejectsInvalidID(t *testing.T) {
	for _, id := range []string{"", "https://example.com/test", "../../etc/passwd", "1234.12345/extra"} {
		w := httptest.NewRecorder()
		API{}.preview(w, httptest.NewRequest("GET", "/feed/preview?arxiv_id="+id, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("id %q: status %d", id, w.Code)
		}
	}
	for _, id := range []string{"2609.12345", "1706.03762v7", "cs/9901001", "math.GT/0309136"} {
		if !previewID.MatchString(id) {
			t.Fatalf("rejected valid ID %q", id)
		}
	}
}

func TestPreviewFetchParseAndCache(t *testing.T) {
	cache := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	defer rdb.Close()
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	calls := 0
	http.DefaultTransport = previewTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := "%PDF-1.7 fixture"
		if r.URL.Host == "parser.test" {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			data, _ := io.ReadAll(file)
			if string(data) != body {
				t.Fatalf("unexpected PDF %q", data)
			}
			body = `{"image":"data:image/png;base64,cG5n","affiliations":["Peking University"]}`
		} else if r.URL.String() != "https://arxiv.org/pdf/2609.12345" {
			t.Fatalf("unexpected upstream %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	a := API{Service: Service{Redis: rdb, ParserURL: "http://parser.test"}}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		a.preview(w, httptest.NewRequest("GET", "/feed/preview?arxiv_id=2609.12345", nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Peking University") {
			t.Fatalf("preview: %d %s", w.Code, w.Body.String())
		}
	}
	if calls != 2 {
		t.Fatalf("cached request fetched upstream: %d calls", calls)
	}
}

func TestPreviewRejectsNonPDF(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = previewTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<html>error</html>")), Header: make(http.Header)}, nil
	})
	if _, err := (Service{}).loadPreview(context.Background(), "2609.12345"); err == nil {
		t.Fatal("accepted an HTML response as PDF")
	}
}
