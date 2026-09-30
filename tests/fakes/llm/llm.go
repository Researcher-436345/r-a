// Package llm is an OpenAI-compatible stand-in for the LLM provider. It
// records every request and answers with a deterministic reply, or with a
// scripted reply/error queued through /_control/script.
package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Script tells the fake how to answer. Match narrows it to requests whose
// messages contain the substring (so parallel callers do not steal each
// other's scripts). Times==0 keeps the script until reset.
type Script struct {
	Match        string `json:"match,omitempty"`
	Reply        string `json:"reply,omitempty"`
	Status       int    `json:"status,omitempty"` // non-2xx → error response with Body
	Body         string `json:"body,omitempty"`   // raw error body for Status
	FinishReason string `json:"finish_reason,omitempty"`
	DelayMS      int    `json:"delay_ms,omitempty"` // before the first byte
	ChunkDelayMS int    `json:"chunk_delay_ms,omitempty"`
	ChunkSize    int    `json:"chunk_size,omitempty"` // runes per stream chunk
	Times        int    `json:"times,omitempty"`
	// Empty answers with no content at all (a reasoning model that spent its
	// whole budget thinking); without it an empty Reply means "echo".
	Empty bool `json:"empty,omitempty"`
	// Sources are emitted as OpenAI url_citation annotations (what a
	// search-capable provider such as Perplexity attaches to its answer).
	Sources []Source `json:"sources,omitempty"`
}

// Source is one cited web page.
type Source struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

func annotationsJSON(sources []Source) []map[string]any {
	if len(sources) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(sources))
	for _, src := range sources {
		out = append(out, map[string]any{"type": "url_citation", "url_citation": map[string]string{"url": src.URL, "title": src.Title}})
	}
	return out
}

// Message mirrors the OpenAI chat message shape (content may be a string or parts).
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func (m Message) Text() string {
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(m.Content, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		return b.String()
	}
	return string(m.Content)
}

// Request is one recorded provider call.
type Request struct {
	At        time.Time      `json:"at"`
	Path      string         `json:"path"`
	Model     string         `json:"model"`
	Stream    bool           `json:"stream"`
	MaxTokens int            `json:"max_tokens"`
	Messages  []Message      `json:"messages"`
	Extra     map[string]any `json:"extra"`
}

type Server struct {
	mu       sync.Mutex
	scripts  []Script
	requests []Request
}

func New() *Server { return &Server{} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "ok", "service": "fake-llm"})
	})
	mux.HandleFunc("/_control/script", s.controlScript)
	mux.HandleFunc("/_control/requests", s.controlRequests)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions") && r.Method == http.MethodPost:
			s.chatCompletions(w, r)
		case strings.HasSuffix(r.URL.Path, "/models") && r.Method == http.MethodGet:
			writeJSON(w, 200, map[string]any{"object": "list", "data": []map[string]any{{"id": "test-model", "object": "model"}}})
		default:
			http.Error(w, "fake-llm: unknown path "+r.URL.Path, 404)
		}
	})
	return mux
}

func (s *Server) controlScript(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodPost:
		var sc Script
		if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if sc.Times == 0 && sc.Match == "" {
			sc.Times = 1 // an unmatched sticky script would hijack every caller
		}
		s.scripts = append(s.scripts, sc)
		writeJSON(w, 201, sc)
	case http.MethodDelete:
		s.scripts = nil
		w.WriteHeader(204)
	case http.MethodGet:
		writeJSON(w, 200, s.scripts)
	default:
		w.WriteHeader(405)
	}
}

func (s *Server) controlRequests(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		match := r.URL.Query().Get("match")
		out := make([]Request, 0, len(s.requests))
		for _, rq := range s.requests {
			if match == "" || contains(rq, match) {
				out = append(out, rq)
			}
		}
		writeJSON(w, 200, out)
	case http.MethodDelete:
		s.requests = nil
		w.WriteHeader(204)
	default:
		w.WriteHeader(405)
	}
}

func contains(rq Request, needle string) bool {
	for _, m := range rq.Messages {
		if strings.Contains(m.Text(), needle) {
			return true
		}
	}
	return false
}

func (s *Server) pick(rq Request) Script {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.scripts {
		sc := s.scripts[i]
		if sc.Match != "" && !contains(rq, sc.Match) {
			continue
		}
		if sc.Times > 0 {
			s.scripts[i].Times--
			if s.scripts[i].Times == 0 {
				s.scripts = append(s.scripts[:i], s.scripts[i+1:]...)
			}
		}
		return sc
	}
	return Script{}
}

func defaultReply(rq Request) string {
	last := ""
	for _, m := range rq.Messages {
		if m.Role == "user" {
			last = m.Text()
		}
	}
	last = strings.Join(strings.Fields(last), " ")
	if len([]rune(last)) > 200 {
		last = string([]rune(last)[:200]) + "…"
	}
	return "[fake-llm] " + last
}

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var body struct {
		Model     string    `json:"model"`
		Stream    bool      `json:"stream"`
		MaxTokens int       `json:"max_tokens"`
		Messages  []Message `json:"messages"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]any{"message": "invalid JSON: " + err.Error()}})
		return
	}
	extra := map[string]any{}
	_ = json.Unmarshal(raw, &extra)
	for _, k := range []string{"model", "stream", "max_tokens", "messages"} {
		delete(extra, k)
	}
	rq := Request{At: time.Now(), Path: r.URL.Path, Model: body.Model, Stream: body.Stream, MaxTokens: body.MaxTokens, Messages: body.Messages, Extra: extra}
	s.mu.Lock()
	s.requests = append(s.requests, rq)
	if len(s.requests) > 500 {
		s.requests = s.requests[len(s.requests)-500:]
	}
	s.mu.Unlock()

	sc := s.pick(rq)
	if sc.DelayMS > 0 {
		time.Sleep(time.Duration(sc.DelayMS) * time.Millisecond)
	}
	if sc.Status != 0 && sc.Status/100 != 2 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(sc.Status)
		if sc.Body == "" {
			sc.Body = fmt.Sprintf(`{"error":{"message":"fake-llm scripted error %d","type":"scripted","code":%d}}`, sc.Status, sc.Status)
		}
		_, _ = w.Write([]byte(sc.Body))
		return
	}
	reply := sc.Reply
	if reply == "" && !sc.Empty {
		reply = defaultReply(rq)
	}
	finish := sc.FinishReason
	if finish == "" {
		finish = "stop"
	}
	if body.Stream {
		s.stream(w, body.Model, reply, finish, sc)
		return
	}
	writeJSON(w, 200, map[string]any{
		"id": "chatcmpl-fake", "object": "chat.completion", "created": time.Now().Unix(), "model": body.Model,
		"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": reply, "annotations": annotationsJSON(sc.Sources)}, "finish_reason": finish}},
		"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": len([]rune(reply)) / 4, "total_tokens": 10 + len([]rune(reply))/4},
	})
}

func (s *Server) stream(w http.ResponseWriter, model, reply, finish string, sc Script) {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)
	size := sc.ChunkSize
	if size <= 0 {
		size = 12
	}
	runes := []rune(reply)
	send := func(delta string, fin any) {
		d := map[string]any{"content": delta}
		if fin != nil && len(sc.Sources) > 0 {
			d["annotations"] = annotationsJSON(sc.Sources)
		}
		chunk := map[string]any{
			"id": "chatcmpl-fake", "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model,
			"choices": []map[string]any{{"index": 0, "delta": d, "finish_reason": fin}},
		}
		b, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		send(string(runes[i:end]), nil)
		if sc.ChunkDelayMS > 0 {
			time.Sleep(time.Duration(sc.ChunkDelayMS) * time.Millisecond)
		}
	}
	send("", finish)
	fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
