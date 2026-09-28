package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/centraluniversity/researcher/internal/platform/config"
)

func TestPaperChatReasoningIsMediumAndScoped(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		requests = append(requests, payload)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Answer\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	llm := LLM{Config: config.Config{LLMProvider: "openai_compatible", LLMAPIKey: "test", LLMBaseURL: server.URL, LLMModel: "test"}, HTTP: server.Client()}
	result, err := llm.ChatWithPaper(context.Background(), testPaper(), "Paper text", "", nil, "Explain a concept beyond this paper", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Reply != "Answer" {
		t.Fatalf("reply: %q", result.Reply)
	}
	reasoning, ok := requests[0]["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "medium" {
		t.Fatalf("reasoning: %#v", requests[0]["reasoning"])
	}
	if _, err := llm.requestStream(context.Background(), "Other task", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := requests[1]["reasoning"]; ok {
		t.Fatal("paper chat changed reasoning for unrelated requests")
	}
}
