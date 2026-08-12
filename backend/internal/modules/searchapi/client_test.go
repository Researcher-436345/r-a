package searchapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestClientStreamsSSEEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search/stream" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: delta\ndata: {\"content\":\"hi\"}\n\nevent: done\ndata: {\"status\":\"ok\"}\n\n"))
	}))
	defer server.Close()
	stream, err := (Client{BaseURL: server.URL}).Stream(context.Background(), []ProviderMessage{{Role: "user", Content: "q"}}, "web")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var names []string
	err = stream.Events(context.Background(), func(event StreamEvent) error {
		names = append(names, event.Name)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"delta", "done"}) {
		t.Fatalf("unexpected events %#v", names)
	}
}
