package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

// FakeLLM drives the provider double.
type FakeLLM struct{ URL string }

func (s Stand) LLM() FakeLLM { return FakeLLM{URL: s.FakeLLMURL} }

// Script mirrors fakes/llm.Script.
type Script struct {
	Match        string `json:"match,omitempty"`
	Reply        string `json:"reply,omitempty"`
	Status       int    `json:"status,omitempty"`
	Body         string `json:"body,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	DelayMS      int    `json:"delay_ms,omitempty"`
	ChunkDelayMS int    `json:"chunk_delay_ms,omitempty"`
	ChunkSize    int    `json:"chunk_size,omitempty"`
	Times        int    `json:"times,omitempty"`
	// Empty forces an answer without content (an empty Reply would mean "echo").
	Empty bool `json:"empty,omitempty"`
	// Sources become url_citation annotations, the way a search-capable
	// provider attaches the pages it cited.
	Sources []Source `json:"sources,omitempty"`
}

// Source mirrors fakes/llm.Source.
type Source struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// LLMRequest mirrors fakes/llm.Request.
type LLMRequest struct {
	Model     string `json:"model"`
	Stream    bool   `json:"stream"`
	MaxTokens int    `json:"max_tokens"`
	Messages  []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	Extra map[string]any `json:"extra"`
}

// Text returns the message content as plain text.
func (r LLMRequest) Text(role string) string {
	var out string
	for _, m := range r.Messages {
		if role != "" && m.Role != role {
			continue
		}
		var s string
		if json.Unmarshal(m.Content, &s) == nil {
			out += s + "\n"
		} else {
			out += string(m.Content) + "\n"
		}
	}
	return out
}

func (f FakeLLM) Script(t testing.TB, sc Script) {
	t.Helper()
	controlPost(t, f.URL+"/_control/script", sc)
}

func (f FakeLLM) Reset(t testing.TB) {
	t.Helper()
	controlDelete(t, f.URL+"/_control/script")
}

// Requests returns recorded provider calls whose messages contain match.
func (f FakeLLM) Requests(t testing.TB, match string) []LLMRequest {
	t.Helper()
	var out []LLMRequest
	controlGet(t, f.URL+"/_control/requests?match="+url.QueryEscape(match), &out)
	return out
}

// FakeScholar drives the arXiv/OpenAlex double.
type FakeScholar struct{ URL string }

func (s Stand) Scholar() FakeScholar { return FakeScholar{URL: s.FakeScholarURL} }

// Rule mirrors fakes/scholar.Rule.
type Rule struct {
	PathPrefix string `json:"path_prefix"`
	Status     int    `json:"status"`
	Body       string `json:"body,omitempty"`
	DelayMS    int    `json:"delay_ms,omitempty"`
	Times      int    `json:"times,omitempty"`
}

func (f FakeScholar) Fail(t testing.TB, r Rule) {
	t.Helper()
	controlPost(t, f.URL+"/_control/rules", r)
}

func (f FakeScholar) Reset(t testing.TB) {
	t.Helper()
	controlDelete(t, f.URL+"/_control/rules")
}

// ScholarRequest mirrors fakes/scholar.Request.
type ScholarRequest struct {
	Method string `json:"method"`
	Host   string `json:"host"`
	Path   string `json:"path"`
	Query  string `json:"query"`
}

// Requests returns recorded upstream calls whose path starts with prefix.
func (f FakeScholar) Requests(t testing.TB, prefix string) []ScholarRequest {
	t.Helper()
	var out []ScholarRequest
	controlGet(t, f.URL+"/_control/requests?path_prefix="+url.QueryEscape(prefix), &out)
	return out
}

// ResetRequests forgets recorded upstream calls.
func (f FakeScholar) ResetRequests(t testing.TB) {
	t.Helper()
	controlDelete(t, f.URL+"/_control/requests")
}

// ResetRequests forgets recorded provider calls.
func (f FakeLLM) ResetRequests(t testing.TB) {
	t.Helper()
	controlDelete(t, f.URL+"/_control/requests")
}

func controlPost(t testing.TB, url string, v any) {
	t.Helper()
	raw, _ := json.Marshal(v)
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("fake control %s: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("fake control %s: status %d", url, resp.StatusCode)
	}
}

func controlDelete(t testing.TB, url string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fake control %s: %v", url, err)
	}
	resp.Body.Close()
}

func controlGet(t testing.TB, url string, v any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("fake control %s: %v", url, err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("fake control %s decode: %v", url, err)
	}
}
