package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"assistant/internal/domain"
	"assistant/internal/qa"
)

type fakeSvc struct {
	prepareFn func(req qa.Request) error
	streamFn  func(onDelta func(string) error) (qa.Result, error)
	historyFn func(chatID string) ([]domain.Message, error)
}

func (f fakeSvc) Prepare(_ context.Context, req qa.Request) (*qa.Prepared, error) {
	if f.prepareFn != nil {
		if err := f.prepareFn(req); err != nil {
			return nil, err
		}
	}
	return &qa.Prepared{}, nil
}

func (f fakeSvc) Stream(_ context.Context, _ *qa.Prepared, onDelta func(string) error) (qa.Result, error) {
	return f.streamFn(onDelta)
}

func (f fakeSvc) History(_ context.Context, chatID string) ([]domain.Message, error) {
	return f.historyFn(chatID)
}

func newTestServer(svc QAService) *httptest.Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httptest.NewServer(NewRouter(svc, logger, 5*time.Second))
}

func TestPostMessageStreamsSSE(t *testing.T) {
	svc := fakeSvc{
		prepareFn: func(req qa.Request) error {
			if req.ChatID != "c1" || req.ArticleID != "a1" {
				t.Errorf("unexpected request: %+v", req)
			}
			return nil
		},
		streamFn: func(onDelta func(string) error) (qa.Result, error) {
			_ = onDelta("Hel")
			_ = onDelta("lo")
			return qa.Result{MessageID: "m1", Answer: "Hello"}, nil
		},
	}
	srv := newTestServer(svc)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/chats/c1/messages", "application/json",
		strings.NewReader(`{"articleId":"a1","content":"hi"}`))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected SSE content type, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	out := string(body)

	for _, want := range []string{"event: delta", `"text":"Hel"`, `"text":"lo"`, "event: done", `"messageId":"m1"`} {
		if !strings.Contains(out, want) {
			t.Errorf("SSE output missing %q; got:\n%s", want, out)
		}
	}
}

func TestPostMessageInvalidJSON(t *testing.T) {
	srv := newTestServer(fakeSvc{})
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/chats/c1/messages", "application/json", strings.NewReader(`not json`))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetMessages(t *testing.T) {
	svc := fakeSvc{
		historyFn: func(chatID string) ([]domain.Message, error) {
			return []domain.Message{{ID: "m1", Role: domain.RoleUser, Content: "hi"}}, nil
		},
	}
	srv := newTestServer(svc)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/chats/c1/messages")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"m1"`) {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(fakeSvc{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
