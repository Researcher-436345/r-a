// Package llm wraps the OpenAI chat-completions API and builds grounded prompts.
package llm

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"

	"assistant/internal/domain"
)

// Client is a thin streaming wrapper over the OpenAI chat-completions API.
type Client struct {
	api         openai.Client
	model       string
	maxTokens   int64
	temperature float64
}

// Options configures the OpenAI client.
type Options struct {
	APIKey      string
	BaseURL     string // optional; empty uses the SDK default
	Model       string
	MaxTokens   int64
	Temperature float64
}

// NewClient builds an OpenAI-backed LLM client. A non-empty BaseURL lets the
// client target OpenAI-compatible gateways.
func NewClient(opts Options) *Client {
	reqOpts := []option.RequestOption{option.WithAPIKey(opts.APIKey)}
	if opts.BaseURL != "" {
		reqOpts = append(reqOpts, option.WithBaseURL(opts.BaseURL))
	}
	return &Client{
		api:         openai.NewClient(reqOpts...),
		model:       opts.Model,
		maxTokens:   opts.MaxTokens,
		temperature: opts.Temperature,
	}
}

// StreamChat streams a completion for the given messages. onDelta is called for
// each non-empty text chunk as it arrives. The full accumulated text is
// returned once the stream completes.
func (c *Client) StreamChat(ctx context.Context, messages []PromptMessage, onDelta func(string) error) (string, error) {
	params := openai.ChatCompletionNewParams{
		Model:       openai.ChatModel(c.model),
		Messages:    toOpenAIMessages(messages),
		Temperature: param.NewOpt(c.temperature),
	}
	if c.maxTokens > 0 {
		params.MaxTokens = param.NewOpt(c.maxTokens)
	}

	stream := c.api.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	var full string
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		full += delta
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return full, err
			}
		}
	}
	if err := stream.Err(); err != nil {
		return full, fmt.Errorf("llm: stream: %w", err)
	}
	return full, nil
}

func toOpenAIMessages(messages []PromptMessage) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case domain.RoleSystem:
			out = append(out, openai.SystemMessage(m.Content))
		case domain.RoleAssistant:
			out = append(out, openai.AssistantMessage(m.Content))
		default:
			out = append(out, openai.UserMessage(m.Content))
		}
	}
	return out
}
