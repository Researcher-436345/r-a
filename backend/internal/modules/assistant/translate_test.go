package assistant

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/translation"
	"github.com/centraluniversity/researcher/internal/platform/config"
)

func TestTranslationClientStreamsDeltas(t *testing.T) {
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("stream") != "1" || r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("unexpected streaming request: %s accept=%q", r.URL.String(), r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintln(w, `data: {"type":"delta","text":"Первая "}`)
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, `data: {"type":"delta","text":"часть"}`)
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, `data: {"type":"done","translation":"Первая часть","target_lang":"ru"}`)
	}))
	defer service.Close()

	client := TranslationClient{Config: config.Config{
		TranslationServiceURL: service.URL,
		LLMTimeout:            time.Second,
	}}
	var streamed strings.Builder
	result, err := client.TranslateStream(
		context.Background(),
		translation.Request{Text: "First part", TargetLang: "ru"},
		func(delta string) error {
			streamed.WriteString(delta)
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if streamed.String() != "Первая часть" || result.Translation != "Первая часть" {
		t.Fatalf("streamed=%q result=%+v", streamed.String(), result)
	}
}
