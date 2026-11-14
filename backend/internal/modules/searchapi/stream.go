package searchapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

func consumeSearchStream(
	ctx context.Context,
	stream *ProviderStream,
	emit func(StreamEvent) error,
	saveResponseID func(string) error,
) (string, []Source, error) {
	var answer strings.Builder
	sources := []Source{}
	completed := false
	err := stream.Events(ctx, func(event StreamEvent) error {
		switch event.Name {
		case "response":
			var response struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(event.Data, &response); err != nil {
				return err
			}
			return saveResponseID(response.ID)
		case "delta", "replace":
			var payload struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return err
			}
			if event.Name == "replace" {
				answer.Reset()
			}
			answer.WriteString(payload.Content)
		case "sources":
			var payload struct {
				Sources []Source `json:"sources"`
			}
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return err
			}
			sources = payload.Sources
		case "error":
			var payload struct {
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return err
			}
			return errors.New(payload.Detail)
		case "done":
			completed = true
			return nil
		case "progress", "source_progress", "ping":
		default:
			return nil
		}
		return emit(event)
	})
	if err != nil {
		return answer.String(), sources, err
	}
	if !completed || strings.TrimSpace(answer.String()) == "" {
		return answer.String(), sources, errors.New("Web search stream ended before completion")
	}
	return answer.String(), sources, nil
}
