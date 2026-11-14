package searchapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProviderMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	BaseURL     string
	Token       string
	HTTP        *http.Client
	WebTimeout  time.Duration
	DeepTimeout time.Duration
}

type ProviderStream struct {
	Response *http.Response
}

func (c Client) Stream(ctx context.Context, messages []ProviderMessage, mode string) (*ProviderStream, error) {
	raw, err := json.Marshal(map[string]any{"messages": messages, "mode": mode})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/search/stream", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.Token != "" {
		req.Header.Set("X-Internal-Token", c.Token)
	}
	client := c.HTTP
	if client == nil {
		timeout := c.WebTimeout
		if mode == "deep" {
			timeout = c.DeepTimeout
		}
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("websearch request: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("websearch returned %s: %s", resp.Status, strings.TrimSpace(string(detail)))
	}
	return &ProviderStream{Response: resp}, nil
}

type ResearchResult struct {
	Status  string   `json:"status"`
	Content string   `json:"content"`
	Sources []Source `json:"sources"`
	Detail  string   `json:"detail"`
}

func (c Client) Result(ctx context.Context, responseID string) (ResearchResult, error) {
	var result ResearchResult
	err := c.responseRequest(ctx, http.MethodGet, responseID, &result)
	return result, err
}

func (c Client) Cancel(ctx context.Context, responseID string) error {
	return c.responseRequest(ctx, http.MethodDelete, responseID, nil)
}

func (c Client) responseRequest(ctx context.Context, method, responseID string, result *ResearchResult) error {
	url := strings.TrimRight(c.BaseURL, "/") + "/v1/search/responses/" + url.PathEscape(responseID)
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("X-Internal-Token", c.Token)
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("websearch response lookup returned %s", resp.Status)
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(result)
	}
	return nil
}

type StreamEvent struct {
	Name string
	Data []byte
}

func (s *ProviderStream) Close() error { return s.Response.Body.Close() }

func (s *ProviderStream) Events(ctx context.Context, onEvent func(StreamEvent) error) error {
	scanner := bufio.NewScanner(s.Response.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	name := "message"
	var data []string
	flush := func() error {
		if len(data) == 0 {
			name = "message"
			return nil
		}
		err := onEvent(StreamEvent{Name: name, Data: []byte(strings.Join(data, "\n"))})
		name = "message"
		data = nil
		return err
	}
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}
