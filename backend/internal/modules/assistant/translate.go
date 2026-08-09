package assistant

import (
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
