package client

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Event is one server-sent event as the frontend would receive it.
type Event struct {
	Name string          // "event:" line (empty for unnamed events)
	Data json.RawMessage // "data:" payload
}

// Type returns the "type" field of a JSON data payload ("" if absent).
func (e Event) Type() string {
	var t struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(e.Data, &t)
	return t.Type
}

// Field returns a string field of the JSON data payload.
func (e Event) Field(name string) string {
	m := map[string]any{}
	_ = json.Unmarshal(e.Data, &m)
	if v, ok := m[name]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		b, _ := json.Marshal(v)
		return string(b)
	}
	return ""
}

// ReadSSE drains a streaming response into events; fails the test when the
// response is not a stream.
func ReadSSE(t testing.TB, resp *http.Response) []Event {
	t.Helper()
	defer resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected an SSE stream, got %s (status %d)", resp.Header.Get("Content-Type"), resp.StatusCode)
	}
	var events []Event
	var cur Event
	var data []string
	flush := func() {
		if len(data) > 0 {
			cur.Data = json.RawMessage(strings.Join(data, "\n"))
			events = append(events, cur)
		}
		cur = Event{}
		data = nil
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "event:"):
			cur.Name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		t.Fatalf("read SSE: %v", err)
	}
	return events
}

// JoinDeltas concatenates the text of delta events (field "text" or "content").
func JoinDeltas(events []Event) string {
	var b strings.Builder
	for _, e := range events {
		if e.Type() == "delta" || e.Name == "delta" {
			if s := e.Field("text"); s != "" {
				b.WriteString(s)
			} else {
				b.WriteString(e.Field("content"))
			}
		}
	}
	return b.String()
}

// Last returns the final event or an empty one.
func Last(events []Event) Event {
	if len(events) == 0 {
		return Event{}
	}
	return events[len(events)-1]
}
