package observability

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestStreamingOutcomeAndTransparentFlushing(t *testing.T) {
	for _, tc := range []struct{ name, body, outcome string }{
		{"complete", "data: {\"type\":\"delta\",\"text\":\"answer\"}\n\ndata: {\"type\":\"done\",\"reply\":\"answer\"}\n\n", "success"},
		{"error_after_200", "data: {\"type\":\"error\",\"detail\":\"provider failed\"}\n\n", "failed"},
		{"premature_eof", "event: delta\ndata: {\"content\":\"partial\"}\n\n", "failed"},
		{"named_done", "event: done\ndata: {\"content\":\"answer\"}\n\n", "success"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			counter := ActionResults.WithLabelValues("test_sse", "summary", tc.outcome, "miss")
			before := testutil.ToFloat64(counter)
			handler := HTTP("test_sse", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, part := range []string{tc.body[:len(tc.body)/2], tc.body[len(tc.body)/2:]} {
					_, _ = io.WriteString(w, part)
					w.(http.Flusher).Flush()
				}
			}))
			r := httptest.NewRequest("POST", "/papers/0199934a-513d-4800-80b2-99be47a02211/summary?stream=1", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 200 || !w.Flushed || w.Body.String() != tc.body {
				t.Fatalf("middleware changed streaming response: status=%d body=%q", w.Code, w.Body.String())
			}
			if got := testutil.ToFloat64(counter) - before; got != 1 {
				t.Fatalf("terminal outcome=%s increment=%v", tc.outcome, got)
			}
		})
	}
}

func TestProgressIsNotFirstContent(t *testing.T) {
	var first bool
	h := HTTP("test_progress", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetFeature(r.Context(), "deep_research")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: progress\ndata: {\"content\":\"working\"}\n\nevent: error\ndata: {\"detail\":\"failed\"}\n\n")
		first = w.(*flushingWriter).firstContent
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/search/chats/0199934a-513d-4800-80b2-99be47a02211/messages", nil))
	if first {
		t.Fatal("progress was treated as an answer")
	}
}

func TestLabelsDoNotIncludeIdentifiersOrQueries(t *testing.T) {
	request := httptest.NewRequest("GET", "/papers/0199934a-513d-4800-80b2-99be47a02211/pdf?private=query", nil)
	label := routeLabel(request)
	if strings.Contains(label, "0199934a") || strings.Contains(label, "private") || strings.Contains(label, "query") {
		t.Fatal("unbounded route label", label)
	}
}

func TestMissingUsageIsReportedRatherThanInvented(t *testing.T) {
	missing := LLMUsage.WithLabelValues("test_missing", "background", "missing_cost")
	before := testutil.ToFloat64(missing)
	UsageJSON(context.Background(), "test_missing", nil)
	if testutil.ToFloat64(missing)-before != 1 {
		t.Fatal("missing provider cost was not reported")
	}
	cost := LLMCost.WithLabelValues("test_reported", "background")
	prior := testutil.ToFloat64(cost)
	UsageJSON(context.Background(), "test_reported", []byte(`{"prompt_tokens":100,"completion_tokens":20,"cost":0.125}`))
	if testutil.ToFloat64(cost)-prior != 0.125 {
		t.Fatal("provider cost was changed")
	}
}
