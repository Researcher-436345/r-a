package observability

import (
	"context"
	"encoding/json"
)

func WithFeature(ctx context.Context, feature string) context.Context {
	return context.WithValue(ctx, operationKey{}, &Operation{Feature: feature})
}
func UsageJSON(ctx context.Context, model string, raw json.RawMessage) {
	var usage struct {
		Input   *int64   `json:"prompt_tokens"`
		Output  *int64   `json:"completion_tokens"`
		Cost    *float64 `json:"cost"`
		Details struct {
			Cached int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	}
	input, output := int64(-1), int64(-1)
	if json.Unmarshal(raw, &usage) == nil {
		if usage.Input != nil {
			input = *usage.Input
		}
		if usage.Output != nil {
			output = *usage.Output
		}
	}
	RecordUsage(ctx, model, input, output, usage.Details.Cached, usage.Cost)
}
