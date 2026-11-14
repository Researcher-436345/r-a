package assistant

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/observability"
)

type LLM struct {
	Config config.Config
	HTTP   *http.Client
	Model  string // optional per-request override
	// MaxTokens caps the reply when > 0. Long structured outputs (the paper
	// overview) set it explicitly because provider defaults are often too low
	// and a reply cut by the provider is reported as ErrLLMTruncated.
	MaxTokens int
	// Per-request OpenAI reasoning override; empty uses the configured default.
	ReasoningEffort string
}

type ChatTurn struct {
	Role    string
	Content string
}

type ContextUsage struct {
	UsedTokens    int     `json:"used_tokens"`
	LimitTokens   int     `json:"limit_tokens"`
	Percent       float64 `json:"percent"`
	PaperTokens   int     `json:"paper_tokens"`
	HistoryTokens int     `json:"history_tokens"`
	HasFullPaper  bool    `json:"has_full_paper"`
	Model         string  `json:"model"`
}

type ChatResult struct {
	Reply   string
	Summary string
	Usage   ContextUsage
}

var (
	ErrLLMNotConfigured = errors.New("LLM is not configured")
	ErrLLMEmpty         = errors.New("LLM returned an empty response")
	// ErrLLMTruncated is returned together with the partial text when the
	// provider stopped the reply at max_output_tokens.
	ErrLLMTruncated = errors.New("LLM reply was cut by the output token limit")
)

func (l LLM) resolvedModel() string {
	if strings.TrimSpace(l.Model) != "" {
		return strings.TrimSpace(l.Model)
	}
	return l.Config.LLMModel
}

func (l LLM) Chat(
	ctx context.Context,
	p catalog.PaperOut,
	history []ChatTurn,
	message, contextText string,
) (string, error) {
	res, err := l.ChatWithPaper(ctx, p, "", "", history, message, contextText)
	return res.Reply, err
}

func (l LLM) Explain(ctx context.Context, p catalog.PaperOut, text, question string) (string, error) {
	if question == "" {
		question = "Explain this fragment simply and in context."
	}
	system := "You are a helpful research assistant. Explain selected paper fragments clearly, briefly, and accurately. Answer in the same language as the user. Use light Markdown (lists, **bold**) when it helps readability."
	user := paperContext(p) + "\n\nFragment:\n" + text + "\n\nQuestion:\n" + question
	return l.request(ctx, system, []ChatTurn{{Role: "user", Content: user}})
}

func (l LLM) request(ctx context.Context, system string, turns []ChatTurn) (reply string, callErr error) {
	observe := observability.ObserveLLM(ctx, l.resolvedModel())
	defer func() { observe(callErr) }()
	return l.responses(ctx, system, turns, false, nil)
}

func (l LLM) requestStream(ctx context.Context, system string, turns []ChatTurn, onDelta func(string) error) (reply string, callErr error) {
	observe := observability.ObserveLLM(ctx, l.resolvedModel())
	defer func() { observe(callErr) }()
	return l.responses(ctx, system, turns, true, onDelta)
}

func formatLLMHTTPError(status int, raw []byte, model string) error {
	body := strings.TrimSpace(string(raw))
	lower := strings.ToLower(body)
	switch {
	case status == http.StatusPaymentRequired ||
		strings.Contains(lower, "insufficient balance") ||
		strings.Contains(lower, "insufficient_quota") ||
		strings.Contains(lower, "payment required"):
		return fmt.Errorf("недостаточно средств на балансе LLM для модели %s — выберите другую модель или пополните счет", model)
	case status == http.StatusTooManyRequests || strings.Contains(lower, "rate limit"):
		return fmt.Errorf("LLM временно перегружен (rate limit) для модели %s — подождите немного или смените модель", model)
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("LLM API ключ отклонен — проверьте OPENAI_API_KEY")
	case status == http.StatusNotFound || strings.Contains(lower, "model") && strings.Contains(lower, "not found"):
		return fmt.Errorf("модель %s недоступна у провайдера — выберите другую", model)
	}
	if body == "" {
		return fmt.Errorf("LLM API error: %s", http.StatusText(status))
	}
	// Keep short: avoid dumping nested JSON blobs into the chat UI.
	if len(body) > 280 {
		body = body[:280] + "…"
	}
	return fmt.Errorf("LLM API error (%d): %s", status, body)
}

func paperContext(p catalog.PaperOut) string {
	names := make([]string, 0, len(p.Authors))
	for _, a := range p.Authors {
		names = append(names, a.Name)
	}
	year := "unknown"
	if p.Year != nil {
		year = fmt.Sprint(*p.Year)
	}
	abstract := "No abstract available."
	if p.Abstract != nil && strings.TrimSpace(*p.Abstract) != "" {
		abstract = *p.Abstract
	}
	return fmt.Sprintf("Title: %s\nAuthors: %s\nYear: %s\nAbstract:\n%s", p.Title, or(strings.Join(names, ", "), "unknown"), year, abstract)
}

func or(s, f string) string {
	if strings.TrimSpace(s) == "" {
		return f
	}
	return s
}

func historyTurns(messages []Message) []ChatTurn {
	out := make([]ChatTurn, 0, len(messages))
	for _, m := range messages {
		content := m.Content
		if m.Role == "user" && m.ContextText != nil && strings.TrimSpace(*m.ContextText) != "" {
			content = content + "\n\n[Attached context]\n" + strings.TrimSpace(*m.ContextText)
		}
		out = append(out, ChatTurn{Role: m.Role, Content: content})
	}
	return out
}
