package qa

import (
	"context"
	"errors"
	"testing"
	"time"

	"assistant/internal/domain"
	"assistant/internal/llm"
)

type fakeArticles struct {
	article domain.Article
	err     error
}

func (f fakeArticles) Get(context.Context, string) (domain.Article, error) {
	return f.article, f.err
}

type appended struct {
	role    domain.Role
	content string
}

type fakeChats struct {
	history  []domain.Message
	getErr   error
	appendFn func(role domain.Role, content string) (domain.Message, error)
	appends  []appended
}

func (f *fakeChats) GetMessages(context.Context, string) ([]domain.Message, error) {
	return f.history, f.getErr
}

func (f *fakeChats) AppendMessage(_ context.Context, _ string, role domain.Role, content string) (domain.Message, error) {
	f.appends = append(f.appends, appended{role: role, content: content})
	if f.appendFn != nil {
		return f.appendFn(role, content)
	}
	return domain.Message{ID: "generated", Role: role, Content: content}, nil
}

type fakeLLM struct {
	chunks   []string
	err      error
	received []llm.PromptMessage
}

func (f *fakeLLM) StreamChat(_ context.Context, messages []llm.PromptMessage, onDelta func(string) error) (string, error) {
	f.received = messages
	var full string
	for _, c := range f.chunks {
		full += c
		if onDelta != nil {
			if err := onDelta(c); err != nil {
				return full, err
			}
		}
	}
	return full, f.err
}

func TestAnswerHappyPath(t *testing.T) {
	articles := fakeArticles{article: domain.Article{ID: "a1", Content: "body"}}
	chats := &fakeChats{history: []domain.Message{{Role: domain.RoleUser, Content: "prev"}}}
	fakeModel := &fakeLLM{chunks: []string{"Hel", "lo"}}
	// assistant append returns a specific id
	chats.appendFn = func(role domain.Role, content string) (domain.Message, error) {
		if role == domain.RoleAssistant {
			return domain.Message{ID: "asst-1", Role: role, Content: content}, nil
		}
		return domain.Message{ID: "user-1", Role: role, Content: content}, nil
	}

	svc := New(articles, chats, fakeModel, time.Second)

	prepared, err := svc.Prepare(context.Background(), Request{ChatID: "c1", ArticleID: "a1", Content: "hi"})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	var streamed string
	res, err := svc.Stream(context.Background(), prepared, func(s string) error {
		streamed += s
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if streamed != "Hello" {
		t.Errorf("streamed = %q, want %q", streamed, "Hello")
	}
	if res.Answer != "Hello" || res.MessageID != "asst-1" {
		t.Errorf("unexpected result: %+v", res)
	}

	// Both user and assistant messages persisted, in order.
	if len(chats.appends) != 2 {
		t.Fatalf("expected 2 appends, got %d: %+v", len(chats.appends), chats.appends)
	}
	if chats.appends[0].role != domain.RoleUser || chats.appends[0].content != "hi" {
		t.Errorf("first append should be user question, got %+v", chats.appends[0])
	}
	if chats.appends[1].role != domain.RoleAssistant || chats.appends[1].content != "Hello" {
		t.Errorf("second append should be assistant answer, got %+v", chats.appends[1])
	}

	// History was read (before persistence) and passed to the prompt: one system
	// message, 1 history turn, new question.
	if len(fakeModel.received) != 3 {
		t.Errorf("expected 3 prompt messages, got %d", len(fakeModel.received))
	}
}

func TestAnswerValidation(t *testing.T) {
	svc := New(fakeArticles{}, &fakeChats{}, &fakeLLM{}, time.Second)
	cases := []Request{
		{ArticleID: "a", Content: "c"}, // no chat
		{ChatID: "c", Content: "c"},    // no article
		{ChatID: "c", ArticleID: "a"},  // no content
	}
	for i, req := range cases {
		if _, err := svc.Prepare(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("case %d: expected ErrInvalidRequest, got %v", i, err)
		}
	}
}

func TestPrepareUpstreamError(t *testing.T) {
	articles := fakeArticles{err: errors.New("article down")}
	svc := New(articles, &fakeChats{}, &fakeLLM{}, time.Second)

	_, err := svc.Prepare(context.Background(), Request{ChatID: "c", ArticleID: "a", Content: "q"})
	if !errors.Is(err, ErrUpstream) {
		t.Errorf("expected ErrUpstream, got %v", err)
	}
}

func TestStreamLLMError(t *testing.T) {
	svc := New(
		fakeArticles{article: domain.Article{Content: "body"}},
		&fakeChats{},
		&fakeLLM{err: errors.New("model exploded")},
		time.Second,
	)

	prepared, err := svc.Prepare(context.Background(), Request{ChatID: "c", ArticleID: "a", Content: "q"})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if _, err := svc.Stream(context.Background(), prepared, nil); !errors.Is(err, ErrLLM) {
		t.Errorf("expected ErrLLM, got %v", err)
	}
}
