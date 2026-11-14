package searchapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func researchTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("SEARCHAPI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SEARCHAPI_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "research_test_" + uuid.New().String()[:8]
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
	if _, err := pool.Exec(ctx, `CREATE TABLE users (id uuid PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{"005_search_chats.sql", "013_search_runs.sql"} {
		raw, err := os.ReadFile("../../../../migrations/" + filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func TestResearchRunIsAtomicAndCompletionIsIdempotent(t *testing.T) {
	pool := researchTestDB(t)
	ctx := context.Background()
	store := Store{DB: pool}
	userID, chatID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id) VALUES($1)`, userID); err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(ctx, userID, chatID, "Research", "deep"); err != nil {
		t.Fatal(err)
	}
	runID, err := store.StartRun(ctx, chatID, "Question")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartRun(ctx, chatID, "Duplicate"); !errors.Is(err, ErrResearchRunning) {
		t.Fatalf("parallel run was accepted: %v", err)
	}
	if err := store.SaveResponseID(ctx, runID, "resp_test"); err != nil {
		t.Fatal(err)
	}
	if run, err := store.LatestRun(ctx, uuid.New(), chatID); err != nil || run != nil {
		t.Fatalf("another user can inspect research: %#v %v", run, err)
	}
	var wait sync.WaitGroup
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := store.CompleteRun(ctx, runID, "Answer", nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wait.Wait()
	messages, err := store.Messages(ctx, chatID, 20)
	if err != nil || len(messages) != 2 || messages[0].Content != "Question" || messages[1].Content != "Answer" {
		t.Fatalf("completion duplicated or lost messages: %#v %v", messages, err)
	}
}

func TestRecoverResearchAfterRestart(t *testing.T) {
	pool := researchTestDB(t)
	ctx := context.Background()
	store := Store{DB: pool}
	userID, chatID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id) VALUES($1)`, userID); err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(ctx, userID, chatID, "Research", "deep"); err != nil {
		t.Fatal(err)
	}
	runID, err := store.StartRun(ctx, chatID, "Question")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveResponseID(ctx, runID, "resp_saved"); err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search/responses/resp_saved" || r.Header.Get("X-Internal-Token") != "internal-test" {
			t.Error("recovery used the wrong response or token")
		}
		fmt.Fprint(w, `{"status":"completed","content":"Recovered report","sources":[]}`)
	}))
	defer provider.Close()
	api := API{DB: pool, Provider: Client{BaseURL: provider.URL, Token: "internal-test"}}
	run, err := api.recoverResearch(ctx, userID, chatID)
	if err != nil || run.Status != "completed" {
		t.Fatalf("recovery failed: %#v %v", run, err)
	}
	if _, err := api.recoverResearch(ctx, userID, chatID); err != nil {
		t.Fatal(err)
	}
	messages, err := store.Messages(ctx, chatID, 20)
	if err != nil || len(messages) != 2 || messages[1].Content != "Recovered report" {
		t.Fatalf("recovered messages: %#v %v", messages, err)
	}
}

func TestDeepResearchPersistsAfterBrowserDisconnect(t *testing.T) {
	pool := researchTestDB(t)
	ctx := context.Background()
	store := Store{DB: pool}
	userID, chatID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id) VALUES($1)`, userID); err != nil {
		t.Fatal(err)
	}
	finish := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response\ndata: {\"id\":\"resp_detached\"}\n\n")
		w.(http.Flusher).Flush()
		<-finish
		fmt.Fprint(w, "event: delta\ndata: {\"content\":\"Unlinked draft\"}\n\nevent: replace\ndata: {\"content\":\"Final linked report\"}\n\nevent: done\ndata: {}\n\n")
	}))
	defer provider.Close()
	router := chi.NewRouter()
	router.Use(identity.MiddlewareFromGateway)
	API{DB: pool, Provider: Client{BaseURL: provider.URL, DeepTimeout: 5 * time.Second}}.Mount(router)
	server := httptest.NewServer(router)
	defer server.Close()
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, server.URL+"/search/chats/"+chatID.String()+"/messages", strings.NewReader(`{"message":"Question","mode":"deep"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(identity.UserIDHeader, userID.String())
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	if err != nil {
		close(finish)
		t.Fatal(err)
	}
	cancel()
	response.Body.Close()
	close(finish)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		run, err := store.LatestRun(ctx, userID, chatID)
		if err != nil {
			t.Fatal(err)
		}
		if run != nil && run.Status == "completed" {
			messages, err := store.Messages(ctx, chatID, 20)
			if err != nil || len(messages) != 2 || messages[1].Content != "Final linked report" {
				t.Fatalf("wrong persisted report: %#v %v", messages, err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("browser disconnect prevented the report from being saved")
}
