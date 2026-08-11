package assistant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/platform/config"
)

type LLM struct {
	Config config.Config
	HTTP   *http.Client
	Model  string // optional per-request override
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

func (l LLM) request(ctx context.Context, system string, turns []ChatTurn) (string, error) {
	if l.Config.LLMProvider == "gemini" {
		return l.gemini(ctx, system, turns)
	}
	return l.openAI(ctx, system, turns)
}

func (l LLM) requestStream(ctx context.Context, system string, turns []ChatTurn, onDelta func(string) error) (string, error) {
	if l.Config.LLMProvider == "gemini" {
		// Gemini path stays non-streaming for now; emit once when complete.
		out, err := l.gemini(ctx, system, turns)
		if err != nil {
			return "", err
		}
		if onDelta != nil {
			if err := onDelta(out); err != nil {
				return out, err
			}
		}
		return out, nil
	}
	return l.openAIStream(ctx, system, turns, onDelta)
}

func (l LLM) openAIMessages(system string, turns []ChatTurn) []map[string]string {
	messages := make([]map[string]string, 0, len(turns)+1)
	messages = append(messages, map[string]string{"role": "system", "content": system})
	for _, turn := range turns {
		role := turn.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		messages = append(messages, map[string]string{"role": role, "content": turn.Content})
	}
	return messages
}

func (l LLM) openAIRequest(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(l.Config.LLMBaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+l.Config.LLMAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if strings.Contains(l.Config.LLMBaseURL, "openrouter.ai") {
		req.Header.Set("HTTP-Referer", l.Config.LLMHTTPReferer)
		req.Header.Set("X-Title", l.Config.LLMAppTitle)
	}
	client := l.HTTP
	if client == nil {
		client = &http.Client{Timeout: l.Config.LLMTimeout}
	}
	return client.Do(req)
}

func (l LLM) openAI(ctx context.Context, system string, turns []ChatTurn) (string, error) {
	if l.Config.LLMAPIKey == "" {
		return "", fmt.Errorf("%w: add LLM_API_KEY to .env (AITunnel recommended from RU)", ErrLLMNotConfigured)
	}
	body, err := json.Marshal(map[string]any{
		"model":       l.resolvedModel(),
		"messages":    l.openAIMessages(system, turns),
		"temperature": 0.2,
	})
	if err != nil {
		return "", err
	}
	resp, err := l.openAIRequest(ctx, body)
	if err != nil {
		return "", fmt.Errorf("LLM API request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", formatLLMHTTPError(resp.StatusCode, raw, l.resolvedModel())
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return "", ErrLLMEmpty
	}
	if s := strings.TrimSpace(out.Choices[0].Message.Content); s != "" {
		return s, nil
	}
	return "", ErrLLMEmpty
}

func (l LLM) openAIStream(ctx context.Context, system string, turns []ChatTurn, onDelta func(string) error) (string, error) {
	if l.Config.LLMAPIKey == "" {
		return "", fmt.Errorf("%w: add LLM_API_KEY to .env (AITunnel recommended from RU)", ErrLLMNotConfigured)
	}
	body, err := json.Marshal(map[string]any{
		"model":       l.resolvedModel(),
		"messages":    l.openAIMessages(system, turns),
		"temperature": 0.2,
		"stream":      true,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(l.Config.LLMBaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+l.Config.LLMAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if strings.Contains(l.Config.LLMBaseURL, "openrouter.ai") {
		req.Header.Set("HTTP-Referer", l.Config.LLMHTTPReferer)
		req.Header.Set("X-Title", l.Config.LLMAppTitle)
	}

	// Streaming responses can exceed the non-stream request timeout.
	client := l.HTTP
	if client == nil {
		timeout := l.Config.LLMTimeout
		if timeout < 5*time.Minute {
			timeout = 5 * time.Minute
		}
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM API request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", formatLLMHTTPError(resp.StatusCode, raw, l.resolvedModel())
	}

	scanner := bufio.NewScanner(resp.Body)
	// Some providers send large SSE frames; default 64K is tight.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var full strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			if payload == "[DONE]" {
				break
			}
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		full.WriteString(delta)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return full.String(), err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if full.Len() > 0 {
			return full.String(), nil
		}
		return "", fmt.Errorf("LLM stream interrupted: %w", err)
	}
	if full.Len() == 0 {
		return "", ErrLLMEmpty
	}
	return full.String(), nil
}

func (l LLM) gemini(ctx context.Context, system string, turns []ChatTurn) (string, error) {
	if l.Config.LLMAPIKey == "" {
		return "", fmt.Errorf("%w: add LLM_API_KEY from Google AI Studio", ErrLLMNotConfigured)
	}
	contents := make([]map[string]any, 0, len(turns))
	for _, turn := range turns {
		role := "user"
		if turn.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]string{{"text": turn.Content}},
		})
	}
	body, err := json.Marshal(map[string]any{
		"system_instruction": map[string]any{"parts": []map[string]string{{"text": system}}},
		"contents":           contents,
		"generationConfig":   map[string]float64{"temperature": 0.2},
	})
	if err != nil {
		return "", err
	}
	u := fmt.Sprintf("%s/models/%s:generateContent?key=%s", strings.TrimRight(l.Config.LLMBaseURL, "/"), l.resolvedModel(), l.Config.LLMAPIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := l.HTTP
	if client == nil {
		client = &http.Client{Timeout: l.Config.LLMTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Gemini API request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", formatLLMHTTPError(resp.StatusCode, raw, l.resolvedModel())
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Candidates) == 0 {
		return "", ErrLLMEmpty
	}
	var parts []string
	for _, p := range out.Candidates[0].Content.Parts {
		parts = append(parts, p.Text)
	}
	if s := strings.TrimSpace(strings.Join(parts, "")); s != "" {
		return s, nil
	}
	return "", ErrLLMEmpty
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
		return fmt.Errorf("LLM API ключ отклонен — проверьте LLM_API_KEY")
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
