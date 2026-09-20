package catalog

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMissCacheKeepsOnlyRepeatableMisses(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cache := newMissCache(time.Minute, 2)
	cache.now = func() time.Time { return now }

	cache.remember("a", errors.New("dial tcp: i/o timeout"))
	cache.remember("b", &sourceFetchError{message: "Failed to fetch arXiv metadata", err: errors.New("503")})
	if cache.get("a") != nil || cache.get("b") != nil {
		t.Fatal("upstream failures must stay retryable")
	}
	cache.remember("c", fmt.Errorf("%w: nothing", errUnsupportedArticleURL))
	cache.remember("d", errNotAPaper)
	if !errors.Is(cache.get("c"), errUnsupportedArticleURL) || !errors.Is(cache.get("d"), errNotAPaper) {
		t.Fatal("not_found and not_a_paper must be remembered")
	}
	now = now.Add(2 * time.Minute)
	if cache.get("c") != nil {
		t.Fatal("entries expire")
	}
	cache.remember("e", errNotAPaper)
	cache.remember("f", errNotAPaper)
	cache.remember("g", errNotAPaper)
	if len(cache.entries) > 2 {
		t.Fatalf("cache grew past its bound: %d", len(cache.entries))
	}
	if cache.get("") != nil {
		t.Fatal("empty key never hits")
	}
}

func TestAddFromURLDoesNotRepeatAMiss(t *testing.T) {
	var calls atomic.Int32
	withFakeWeb(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.NotFound(w, r)
	})
	api := API{}
	body := `{"url":"https://blog.example.org/posts/resolve-cache-test","title_hint":"","add_to_library":false}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/papers/from-url", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		api.addFromURL(rec, req)
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
			t.Fatalf("attempt %d: got %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("the page was fetched %d times, want 1", got)
	}
}
