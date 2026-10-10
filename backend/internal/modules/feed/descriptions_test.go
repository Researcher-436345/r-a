package feed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/centraluniversity/researcher/internal/modules/translation"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func descriptionTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("FEED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("FEED_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "feed_test_" + uuid.New().String()[:8]
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE")
		admin.Close()
	})
	migration, err := os.ReadFile("../../../../migrations/010_feed_briefs.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestDescriptionPersistenceAndAtomicPublication(t *testing.T) {
	pool := descriptionTestDB(t)
	ctx := context.Background()
	var calls atomic.Int32
	var fail atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		var request struct {
			Model     string `json:"model"`
			Stream    bool   `json:"stream"`
			Reasoning struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Model != "google/gemma-4-26b-a4b-it" || request.Stream || request.Reasoning.Effort != "none" {
			t.Errorf("unexpected provider request: %+v", request)
		}
		fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"content":"Краткий обзор результатов исследования."}}]}`)
	}))
	defer provider.Close()
	cfg := config.Config{FeedTranslationModel: "google/gemma-4-26b-a4b-it", FeedTranslationBaseURL: provider.URL, FeedTranslationAPIKey: "test", LLMTimeout: time.Second}
	store := PGDescriptionStore{DB: pool}
	cache := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	defer rdb.Close()
	source := []TrendingPaper{{ArxivID: "2609.00001", Abstract: strPtr("An original abstract.")}}
	seed := func(items []TrendingPaper) {
		raw, _ := json.Marshal(items)
		for _, mode := range []SortMode{SortNew, SortHot, SortPopular} {
			for _, limit := range []int{20, 50} {
				if err := rdb.Set(ctx, fmt.Sprintf("feed:trending:v3:cs.AI:%s:%d", mode, limit), raw, time.Hour).Err(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	seed(source)
	service := Service{Redis: rdb, Descriptions: NewDescriptions(cfg, store)}
	if _, _, err := service.LocalizedTrending(ctx, "cs.AI", 20, SortNew, "ru"); !errors.Is(err, ErrFeedPreparing) {
		t.Fatalf("cold feed must not leak English: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("read triggered a model call")
	}
	if err := service.refreshDescriptions(ctx); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("same abstract translated per sort: %d", calls.Load())
	}
	english, _, err := service.LocalizedTrending(ctx, "cs.AI", 20, SortNew, "en")
	if err != nil || len(english) != 1 || *english[0].Abstract != "An original abstract." || calls.Load() != 1 {
		t.Fatalf("English must retain the original abstract: %+v, %v", english, err)
	}
	// Новый экземпляр сервиса, без Redis: все пользователи читают готовую ленту из БД.
	restarted := Service{Descriptions: NewDescriptions(cfg, store)}
	for i := 0; i < 3; i++ {
		papers, cached, err := restarted.LocalizedTrending(ctx, "cs.AI", 20, SortNew, "ru")
		if err != nil || !cached || len(papers) != 1 || *papers[0].Abstract != "Краткий обзор результатов исследования." {
			t.Fatalf("shared snapshot: %+v, %v", papers, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("read after restart called the model")
	}
	if *source[0].Abstract != "An original abstract." {
		t.Fatal("source was mutated")
	}
	// Новая версия аннотации требует перевода, старая готовая лента переживает сбой.
	source[0].Abstract = strPtr("A revised abstract.")
	seed(source)
	fail.Store(true)
	if err := service.refreshDescriptions(ctx); err == nil {
		t.Fatal("expected provider failure")
	}
	papers, _, err := restarted.LocalizedTrending(ctx, "cs.AI", 20, SortNew, "ru")
	if err != nil || *papers[0].Abstract != "Краткий обзор результатов исследования." {
		t.Fatal("failed refresh replaced snapshot")
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM feed_briefs`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("failure was cached: %d, %v", rows, err)
	}
	fail.Store(false)
	if err := service.refreshDescriptions(ctx); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("revised source translated more than once: %d", calls.Load())
	}
	if abstractHash("A  revised\nabstract.") != abstractHash("A revised abstract.") {
		t.Fatal("whitespace should not retrigger translation")
	}
}

func TestRefreshLockAcrossConnections(t *testing.T) {
	store := PGDescriptionStore{DB: descriptionTestDB(t)}
	ctx := context.Background()
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- store.WithRefreshLock(ctx, func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("lock timeout")
	}
	var secondRan bool
	err := store.WithRefreshLock(ctx, func(context.Context) error { secondRan = true; return nil })
	close(release)
	if err != nil || secondRan {
		t.Fatalf("duplicate worker entered: %v, %v", secondRan, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := store.WithRefreshLock(ctx, func(context.Context) error { secondRan = true; return nil }); err != nil || !secondRan {
		t.Fatal("lock was not released")
	}
}

func TestFeedPreparingHTTPContract(t *testing.T) {
	w := httptest.NewRecorder()
	API{}.trending(w, httptest.NewRequest("GET", "/feed/trending?lang=ru", nil))
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 503 || body.Code != "feed_preparing" || w.Header().Get("Retry-After") != "5" {
		t.Fatalf("cold feed response: %d %s", w.Code, w.Body.String())
	}
}

func TestTruncatedTranslationIsNotAccepted(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"finish_reason":"length","message":{"content":"Незаконченный перевод"}}]}`)
	}))
	defer provider.Close()
	d := NewDescriptions(config.Config{FeedTranslationModel: "google/gemma-4-26b-a4b-it", FeedTranslationBaseURL: provider.URL, FeedTranslationAPIKey: "test", LLMTimeout: time.Second}, nil)
	_, err := d.Generator.Generate(context.Background(), "A title", "An abstract")
	if !errors.Is(err, translation.ErrIncompleteResponse) {
		t.Fatalf("accepted truncated translation: %v", err)
	}
}

func TestFeedModelDoesNotFollowChatModel(t *testing.T) {
	t.Setenv("LLM_MODEL", "a-chat-model")
	t.Setenv("TRANSLATION_LLM_MODEL", "a-reader-translation-model")
	t.Setenv("FEED_TRANSLATION_MODEL", "")
	t.Setenv("LLM_API_KEY", "shared-test-key")
	t.Setenv("FEED_TRANSLATION_API_KEY", "")
	cfg := config.Load()
	if cfg.FeedTranslationModel != "google/gemma-4-26b-a4b-it" || cfg.FeedTranslationAPIKey != "shared-test-key" {
		t.Fatal("feed must use Gemma and inherit the configured key")
	}
}

func TestBriefValidation(t *testing.T) {
	for _, text := range []string{"", "An English summary of the research.", "Краткий обзор: https://example.com", strings.Repeat("я", 281)} {
		if _, err := validateBrief(text); err == nil {
			t.Fatalf("accepted invalid brief: %.30s", text)
		}
	}
	text, err := validateBrief("Авторы разработали метод.\n\nОн улучшает точность классификации.")
	if err != nil || strings.Contains(text, "\n") {
		t.Fatalf("normalization: %q, %v", text, err)
	}
}

func TestOverlongBriefRetriesWithShorterInstruction(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := strings.Repeat("я", 281)
		if calls.Add(1) == 2 {
			content = "Авторы разработали метод ускорения анимации 3D-аватаров с сохранением качества."
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}}})
	}))
	defer provider.Close()
	g := gemmaBriefGenerator{Config: config.Config{FeedTranslationModel: "google/gemma-4-26b-a4b-it", FeedTranslationBaseURL: provider.URL, FeedTranslationAPIKey: "test", LLMTimeout: time.Second}}
	text, err := g.Generate(context.Background(), "Title", "Abstract")
	if err != nil || calls.Load() != 2 || len(text) == 0 {
		t.Fatalf("shorter retry: %q, %d, %v", text, calls.Load(), err)
	}
}
