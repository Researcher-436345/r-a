package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"assistant/internal/domain"
)

func TestGetMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/chats/c1/messages" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":"m1","role":"user","content":"hi","createdAt":"t"}]`))
	}))
	defer srv.Close()

	c := New(srv.URL, srv.Client())
	msgs, err := c.GetMessages(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Role != domain.RoleUser {
		t.Errorf("unexpected messages: %+v", msgs)
	}
}

func TestAppendMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var in struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		_ = json.Unmarshal(body, &in)
		if in.Role != "assistant" || in.Content != "answer" {
			t.Errorf("unexpected body: %s", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"m2","role":"assistant","content":"answer","createdAt":"t"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, srv.Client())
	msg, err := c.AppendMessage(context.Background(), "c1", domain.RoleAssistant, "answer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ID != "m2" {
		t.Errorf("unexpected created message: %+v", msg)
	}
}

func TestGetMessagesNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, srv.Client())
	if _, err := c.GetMessages(context.Background(), "c1"); err == nil {
		t.Fatal("expected error for 500")
	}
}
