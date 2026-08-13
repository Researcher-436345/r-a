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

	"github.com/centraluniversity/researcher/internal/modules/translation"
	"github.com/centraluniversity/researcher/internal/platform/config"
)

type TranslationClient struct {
	Config config.Config
	HTTP   *http.Client
}

type TranslationServiceError struct {
	Status int
	Detail string
}

func (e *TranslationServiceError) Error() string { return e.Detail }

func (c TranslationClient) Translate(ctx context.Context, input translation.Request) (translation.Response, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return translation.Response{}, fmt.Errorf("encode translation request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.Config.TranslationServiceURL, "/")+"/translate", bytes.NewReader(body))
	if err != nil {
		return translation.Response{}, fmt.Errorf("create translation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: c.Config.LLMTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return translation.Response{}, fmt.Errorf("translation service unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		var payload struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&payload)
		if payload.Detail == "" {
			payload.Detail = "Translation service is unavailable"
		}
		return translation.Response{}, &TranslationServiceError{Status: resp.StatusCode, Detail: payload.Detail}
	}
	var result translation.Response
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil || strings.TrimSpace(result.Translation) == "" {
		return translation.Response{}, fmt.Errorf("translation service returned an invalid response")
	}
	return result, nil
}

func (c TranslationClient) TranslateStream(
	ctx context.Context,
	input translation.Request,
	onDelta func(string) error,
) (translation.Response, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return translation.Response{}, fmt.Errorf("encode translation request: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.Config.TranslationServiceURL, "/")+"/translate?stream=1",
		bytes.NewReader(body),
	)
	if err != nil {
		return translation.Response{}, fmt.Errorf("create translation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: c.Config.LLMTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return translation.Response{}, fmt.Errorf("translation service unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		var payload struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&payload)
		if payload.Detail == "" {
			payload.Detail = "Translation service is unavailable"
		}
		return translation.Response{}, &TranslationServiceError{Status: resp.StatusCode, Detail: payload.Detail}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	var result translation.Response
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var event translation.StreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		switch event.Type {
		case "delta":
			if event.Text != "" && onDelta != nil {
				if err := onDelta(event.Text); err != nil {
					return translation.Response{}, err
				}
			}
		case "done":
			result = translation.Response{Translation: event.Translation, TargetLang: event.TargetLang}
		case "error":
			return translation.Response{}, &TranslationServiceError{Status: http.StatusBadGateway, Detail: event.Detail}
		}
	}
	if err := scanner.Err(); err != nil {
		return translation.Response{}, fmt.Errorf("translation stream interrupted: %w", err)
	}
	if strings.TrimSpace(result.Translation) == "" {
		return translation.Response{}, fmt.Errorf("translation stream ended without completion")
	}
	return result, nil
}
