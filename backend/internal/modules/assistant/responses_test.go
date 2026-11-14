package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/centraluniversity/researcher/internal/platform/config"
)

func TestResponsesUsesNativeParametersAndMessageOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer openai-test" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["instructions"] != "System" || body["max_output_tokens"] != float64(1234) || body["store"] != false {
			t.Errorf("wrong Responses payload: %#v", body)
		}
		for _, obsolete := range []string{"temperature", "max_tokens", "messages"} {
			if _, exists := body[obsolete]; exists {
				t.Errorf("obsolete parameter: %s", obsolete)
			}
		}
		fmt.Fprint(w, `{"status":"completed","output":[{"type":"reasoning","summary":[]},{"type":"message","content":[{"type":"output_text","text":"Answer"}]}],"usage":{"input_tokens":20,"output_tokens":5}}`)
	}))
	defer server.Close()
	llm := LLM{Config: config.Config{
		LLMBaseURL: server.URL, LLMAPIKey: "openai-test", LLMModel: "test-model", LLMMaxOutputTokens: 1234,
	}}
	out, err := llm.request(context.Background(), "System", []ChatTurn{{Role: "user", Content: "Question"}})
	if err != nil || out != "Answer" {
		t.Fatalf("response=%q err=%v", out, err)
	}
}

func TestResponseStreamRequiresTerminalSuccess(t *testing.T) {
	for _, test := range []struct {
		name, terminal string
		truncated      bool
	}{
		{"missing_terminal", "", false},
		{"token_limit", `data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`, true},
		{"provider_failed", `data: {"type":"response.failed","response":{"status":"failed","error":{"message":"failed"}}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"Partial"}`+"\n\n"+test.terminal+"\n\n")
			}))
			defer server.Close()
			llm := LLM{Config: config.Config{LLMBaseURL: server.URL, LLMAPIKey: "test", LLMModel: "test"}}
			out, err := llm.requestStream(context.Background(), "", nil, nil)
			if err == nil || out != "Partial" || errors.Is(err, ErrLLMTruncated) != test.truncated {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}
