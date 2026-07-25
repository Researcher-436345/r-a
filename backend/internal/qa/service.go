// Package qa orchestrates answering a question about an article: it gathers the
// article and chat history, streams an LLM answer, and persists the exchange.
package qa

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"assistant/internal/domain"
	"assistant/internal/llm"
)

// ArticleFetcher retrieves article content by ID.
type ArticleFetcher interface {
	Get(ctx context.Context, articleID string) (domain.Article, error)
}

// ChatStore reads and appends chat messages.
type ChatStore interface {
	GetMessages(ctx context.Context, chatID string) ([]domain.Message, error)
	AppendMessage(ctx context.Context, chatID string, role domain.Role, content string) (domain.Message, error)
}

// LLMStreamer streams an answer, invoking onDelta for each text chunk and
// returning the full text.
type LLMStreamer interface {
	StreamChat(ctx context.Context, messages []llm.PromptMessage, onDelta func(string) error) (string, error)
}

// Service ties the upstream services and the LLM together.
type Service struct {
	articles     ArticleFetcher
	chats        ChatStore
	llm          LLMStreamer
	persistLimit time.Duration
}

// New builds a qa Service. persistTimeout bounds the post-stream save that runs
// on a background context so a client disconnect doesn't lose the answer.
func New(articles ArticleFetcher, chats ChatStore, streamer LLMStreamer, persistTimeout time.Duration) *Service {
	return &Service{
		articles:     articles,
		chats:        chats,
		llm:          streamer,
		persistLimit: persistTimeout,
	}
}

// Request is a question about an article within a chat.
type Request struct {
	ChatID    string
	ArticleID string
	Content   string
}

// Result reports the outcome after the answer has been streamed and saved.
type Result struct {
	MessageID string
	Answer    string
}

// Prepared holds everything gathered before streaming starts.
type Prepared struct {
	req      Request
	messages []llm.PromptMessage
}

// Prepare validates the request and gathers the article and chat history. These
// steps run before any streaming, so their failures (ErrInvalidRequest,
// ErrUpstream) can be surfaced as a normal HTTP status.
func (s *Service) Prepare(ctx context.Context, req Request) (*Prepared, error) {
	if req.ChatID == "" {
		return nil, fmt.Errorf("%w: chatId is required", ErrInvalidRequest)
	}
	if req.ArticleID == "" {
		return nil, fmt.Errorf("%w: articleId is required", ErrInvalidRequest)
	}
	if req.Content == "" {
		return nil, fmt.Errorf("%w: content is required", ErrInvalidRequest)
	}

	// Fetch the article and prior history concurrently. History is read before
	// we persist the new question so it isn't duplicated in the prompt.
	var (
		article    domain.Article
		history    []domain.Message
		articleErr error
		historyErr error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		article, articleErr = s.articles.Get(ctx, req.ArticleID)
	}()
	go func() {
		defer wg.Done()
		history, historyErr = s.chats.GetMessages(ctx, req.ChatID)
	}()
	wg.Wait()
	if err := errors.Join(articleErr, historyErr); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	return &Prepared{
		req:      req,
		messages: llm.BuildPrompt(article, history, req.Content),
	}, nil
}

// Stream runs the LLM completion for a prepared request, forwarding each chunk
// to onDelta, then persists the user question and assistant answer. It is called
// after the SSE stream has started, so its errors are reported as SSE events.
//
// Persistence uses a fresh background context (bounded by persistTimeout) so the
// exchange is saved even if the client's request context is already cancelled.
func (s *Service) Stream(ctx context.Context, p *Prepared, onDelta func(string) error) (Result, error) {
	answer, err := s.llm.StreamChat(ctx, p.messages, onDelta)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrLLM, err)
	}

	assistantMsg, err := s.persist(p.req, answer)
	if err != nil {
		return Result{}, err
	}

	return Result{MessageID: assistantMsg.ID, Answer: answer}, nil
}

func (s *Service) persist(req Request, answer string) (domain.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.persistLimit)
	defer cancel()

	if _, err := s.chats.AppendMessage(ctx, req.ChatID, domain.RoleUser, req.Content); err != nil {
		return domain.Message{}, fmt.Errorf("%w: save user message: %v", ErrPersist, err)
	}
	assistantMsg, err := s.chats.AppendMessage(ctx, req.ChatID, domain.RoleAssistant, answer)
	if err != nil {
		return domain.Message{}, fmt.Errorf("%w: save assistant message: %v", ErrPersist, err)
	}
	return assistantMsg, nil
}

// History returns the stored messages for a chat.
func (s *Service) History(ctx context.Context, chatID string) ([]domain.Message, error) {
	if chatID == "" {
		return nil, fmt.Errorf("%w: chatId is required", ErrInvalidRequest)
	}
	msgs, err := s.chats.GetMessages(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	return msgs, nil
}

// Sentinel errors let the HTTP layer map failures to status codes.
var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrUpstream       = errors.New("upstream service error")
	ErrLLM            = errors.New("llm error")
	ErrPersist        = errors.New("persist error")
)
