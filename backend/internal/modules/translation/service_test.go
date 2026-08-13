package translation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/internal/platform/config"
)

func testConfig(baseURL string) config.Config {
	return config.Config{
		LLMBaseURL:          baseURL,
		LLMAPIKey:           "secret-key",
		LLMModel:            "test-model",
		LLMTimeout:          time.Second,
		TranslationMaxChars: 20,
	}
}

func TestTranslateCallsOpenAICompatibleProvider(t *testing.T) {
	var gotPath, gotAuth, gotText string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotText = body.Messages[1].Content
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Привет, мир!"}}]}`))
	}))
	defer provider.Close()

	result, err := (Service{Config: testConfig(provider.URL)}).Translate(context.Background(), Request{
		Text: "Hello, world!", TargetLang: "ru",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Translation != "Привет, мир!" || result.TargetLang != "ru" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotPath != "/chat/completions" || gotAuth != "Bearer secret-key" {
		t.Fatalf("path=%q auth=%q", gotPath, gotAuth)
	}
	if !strings.Contains(gotText, "<source_text>\nHello, world!\n</source_text>") {
		t.Fatalf("selected text was not delimited: %q", gotText)
	}
}

func TestValidateRejectsLongTextAndUnknownLanguage(t *testing.T) {
	_, err := Validate(Request{Text: "123456", TargetLang: "ru"}, 5)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || !strings.Contains(err.Error(), "maximum is 5") {
		t.Fatalf("unexpected long-text error: %v", err)
	}
	_, err = Validate(Request{Text: "hello", TargetLang: "made-up"}, 10)
	if !errors.As(err, &validationErr) || err.Error() != "Unsupported target language" {
		t.Fatalf("unexpected language error: %v", err)
	}
}

func TestTranslateDoesNotLeakProviderErrorBody(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "sensitive upstream detail", http.StatusUnauthorized)
	}))
	defer provider.Close()

	_, err := (Service{Config: testConfig(provider.URL)}).Translate(context.Background(), Request{Text: "hello", TargetLang: "ru"})
	if !errors.Is(err, ErrProvider) {
		t.Fatalf("expected provider error, got %v", err)
	}
	if strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("provider body leaked: %v", err)
	}
}

func TestTranslateStreamUsesDedicatedModelWithoutReasoning(t *testing.T) {
	var gotModel, gotEffort string
	var gotStream bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model     string `json:"model"`
			Stream    bool   `json:"stream"`
			Reasoning struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotModel, gotStream, gotEffort = body.Model, body.Stream, body.Reasoning.Effort
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Привет, "}}]}`)
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"мир!"}}]}`)
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "data: [DONE]")
	}))
	defer provider.Close()

	cfg := testConfig(provider.URL)
	cfg.TranslationLLMModel = "google/gemma-4-26b-a4b-it"
	var streamed strings.Builder
	result, err := (Service{Config: cfg}).TranslateStream(
		context.Background(),
		Request{Text: "Hello, world!", TargetLang: "ru"},
		func(delta string) error {
			streamed.WriteString(delta)
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotModel != cfg.TranslationLLMModel || !gotStream || gotEffort != "none" {
		t.Fatalf("model=%q stream=%v effort=%q", gotModel, gotStream, gotEffort)
	}
	if streamed.String() != "Привет, мир!" || result.Translation != "Привет, мир!" {
		t.Fatalf("streamed=%q result=%+v", streamed.String(), result)
	}
}
