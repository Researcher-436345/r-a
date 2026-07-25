// Package chat provides an HTTP client for the upstream Chat Service, which
// stores conversation messages.
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"assistant/internal/domain"
)

// Client reads and appends chat messages via the Chat Service.
//
// Assumed contract:
//
//	GET  {baseURL}/chats/{chatID}/messages -> [{"id","role","content","createdAt"}]
//	POST {baseURL}/chats/{chatID}/messages  body {"role","content"} -> created message
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a Chat Service client. httpClient must be non-nil.
func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
	}
}

// GetMessages returns the conversation history for a chat, oldest first.
func (c *Client) GetMessages(ctx context.Context, chatID string) ([]domain.Message, error) {
	endpoint := fmt.Sprintf("%s/chats/%s/messages", c.baseURL, url.PathEscape(chatID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("chat: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat: get messages %s: %w", chatID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, statusError("chat: get messages", chatID, resp)
	}

	var msgs []domain.Message
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		return nil, fmt.Errorf("chat: decode messages %s: %w", chatID, err)
	}
	return msgs, nil
}

// AppendMessage persists a message to a chat and returns the created record.
func (c *Client) AppendMessage(ctx context.Context, chatID string, role domain.Role, content string) (domain.Message, error) {
	endpoint := fmt.Sprintf("%s/chats/%s/messages", c.baseURL, url.PathEscape(chatID))

	payload, err := json.Marshal(struct {
		Role    domain.Role `json:"role"`
		Content string      `json:"content"`
	}{Role: role, Content: content})
	if err != nil {
		return domain.Message{}, fmt.Errorf("chat: encode message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return domain.Message{}, fmt.Errorf("chat: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.Message{}, fmt.Errorf("chat: append message %s: %w", chatID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return domain.Message{}, statusError("chat: append message", chatID, resp)
	}

	var msg domain.Message
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return domain.Message{}, fmt.Errorf("chat: decode created message %s: %w", chatID, err)
	}
	return msg, nil
}

func statusError(op, chatID string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("%s %s returned %d: %s", op, chatID, resp.StatusCode, strings.TrimSpace(string(body)))
}
