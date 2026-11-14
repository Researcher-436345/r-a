package assistant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/platform/observability"
)

type modelResponse struct {
	Status string `json:"status"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage json.RawMessage `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	IncompleteDetails struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
}

func (r modelResponse) completionError() error {
	switch r.Status {
	case "completed":
		return nil
	case "incomplete":
		if r.IncompleteDetails.Reason == "max_output_tokens" {
			return ErrLLMTruncated
		}
		return fmt.Errorf("OpenAI response incomplete: %s", r.IncompleteDetails.Reason)
	case "failed":
		return fmt.Errorf("OpenAI response failed: %s", r.Error.Message)
	default:
		return fmt.Errorf("OpenAI response did not complete: %s", r.Status)
	}
}

func (l LLM) responsesPayload(system string, turns []ChatTurn, stream bool) (map[string]any, error) {
	if l.Config.LLMProvider != "" && l.Config.LLMProvider != "openai" {
		return nil, fmt.Errorf("%w: set LLM_PROVIDER=openai for Responses API", ErrLLMNotConfigured)
	}
	if strings.TrimSpace(l.Config.LLMAPIKey) == "" {
		return nil, fmt.Errorf("%w: set OPENAI_API_KEY", ErrLLMNotConfigured)
	}
	if l.resolvedModel() == "" {
		return nil, fmt.Errorf("%w: select LLM_MODEL in settings.py or env", ErrLLMNotConfigured)
	}
	input := make([]map[string]string, 0, len(turns))
	for _, turn := range turns {
		if turn.Role != "user" && turn.Role != "assistant" {
			return nil, fmt.Errorf("unsupported conversation role: %s", turn.Role)
		}
		input = append(input, map[string]string{"role": turn.Role, "content": turn.Content})
	}
	payload := map[string]any{
		"model": l.resolvedModel(), "instructions": system, "input": input,
		"stream": stream, "store": false,
	}
	effort := l.ReasoningEffort
	if effort == "" {
		effort = l.Config.LLMReasoningEffort
	}
	if effort != "" {
		payload["reasoning"] = map[string]string{"effort": effort}
	}
	maxTokens := l.MaxTokens
	if maxTokens <= 0 {
		maxTokens = l.Config.LLMMaxOutputTokens
	}
	if maxTokens > 0 {
		payload["max_output_tokens"] = maxTokens
	}
	return payload, nil
}

func (l LLM) responses(ctx context.Context, system string, turns []ChatTurn, stream bool, onDelta func(string) error) (string, error) {
	payload, err := l.responsesPayload(system, turns, stream)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(l.Config.LLMBaseURL, "/") + "/responses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+l.Config.LLMAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	client := l.HTTP
	if client == nil {
		client = &http.Client{Timeout: l.Config.LLMTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()
	observability.ProviderResponse(ctx, l.resolvedModel(), resp.StatusCode)
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", formatLLMHTTPError(resp.StatusCode, raw, l.resolvedModel())
	}
	if stream {
		return l.readResponseStream(ctx, resp.Body, onDelta)
	}
	var out modelResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("decode OpenAI response: %w", err)
	}
	observability.UsageJSON(ctx, l.resolvedModel(), out.Usage)
	var reply strings.Builder
	for _, item := range out.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type == "output_text" {
				reply.WriteString(content.Text)
			}
		}
	}
	if err := out.completionError(); err != nil {
		return reply.String(), err
	}
	if strings.TrimSpace(reply.String()) == "" {
		return "", ErrLLMEmpty
	}
	return strings.TrimSpace(reply.String()), nil
}

func (l LLM) readResponseStream(ctx context.Context, body io.Reader, onDelta func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 8<<20)
	var reply strings.Builder
	var data []string
	completed := false
	var reportedUsage json.RawMessage
	defer func() { observability.UsageJSON(ctx, l.resolvedModel(), reportedUsage) }()
	handleEvent := func() error {
		if len(data) == 0 {
			return nil
		}
		var event struct {
			Type     string        `json:"type"`
			Delta    string        `json:"delta"`
			Message  string        `json:"message"`
			Response modelResponse `json:"response"`
		}
		if err := json.Unmarshal([]byte(strings.Join(data, "\n")), &event); err != nil {
			return fmt.Errorf("decode OpenAI stream event: %w", err)
		}
		data = nil
		switch event.Type {
		case "response.output_text.delta":
			reply.WriteString(event.Delta)
			if onDelta != nil {
				return onDelta(event.Delta)
			}
		case "response.completed", "response.incomplete", "response.failed":
			reportedUsage = event.Response.Usage
			completed = true
			return event.Response.completionError()
		case "error":
			return fmt.Errorf("OpenAI stream error: %s", event.Message)
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := handleEvent(); err != nil {
				return reply.String(), err
			}
			if completed {
				break
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return reply.String(), fmt.Errorf("OpenAI stream interrupted: %w", err)
	}
	if err := handleEvent(); err != nil {
		return reply.String(), err
	}
	if !completed {
		return reply.String(), fmt.Errorf("OpenAI stream ended before response.completed")
	}
	if strings.TrimSpace(reply.String()) == "" {
		return "", ErrLLMEmpty
	}
	return reply.String(), nil
}
