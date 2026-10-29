package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const ActionHeader = "X-Research-Action-ID"
const SessionHeader = "X-Research-Session-ID"

type operationKey struct{}

// Operation contains metadata only: prompts, answers, tokens and IPs are never copied here.
type Operation struct {
	InputSource                                         string
	ID, UserID, SessionID, PaperID, ProjectID, ResultID string
	Feature, Locale, Device, Source, Outcome            string
	CacheHit                                            bool
	StartedAt, FinishedAt                               time.Time
}

var observer struct {
	sync.RWMutex
	fn func(Operation, string)
}

func SetObserver(fn func(Operation, string)) { observer.Lock(); observer.fn = fn; observer.Unlock() }
func notify(op *Operation, stage string) {
	observer.RLock()
	fn := observer.fn
	observer.RUnlock()
	if fn != nil && op.UserID != "" {
		fn(*op, stage)
	}
}

func BindUser(ctx context.Context, id string) {
	if op, ok := ctx.Value(operationKey{}).(*Operation); ok && op.UserID == "" && id != "" && id != uuid.Nil.String() {
		op.UserID = id
		notify(op, "started")
	}
}

func SetFeature(ctx context.Context, feature string) {
	if op, ok := ctx.Value(operationKey{}).(*Operation); ok && validFeature(feature) {
		op.Feature = feature
	}
}
func CacheHit(ctx context.Context, cache string) {
	CacheRequests.WithLabelValues(cache, "hit").Inc()
	if op, ok := ctx.Value(operationKey{}).(*Operation); ok {
		op.CacheHit = true
	}
}
func CacheMiss(cache string) { CacheRequests.WithLabelValues(cache, "miss").Inc() }

func validFeature(value string) bool {
	switch value {
	case "summary", "translation", "chat", "explanation", "search", "deep_research":
		return true
	}
	return false
}

func canonicalUUID(value string) string {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return ""
	}
	return id.String()
}

func requestOperation(r *http.Request) *Operation {
	if r.Method != http.MethodPost {
		return nil
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	feature, paper := "", ""
	if len(parts) == 3 && parts[0] == "papers" && canonicalUUID(parts[1]) != "" {
		paper = canonicalUUID(parts[1])
		switch parts[2] {
		case "summary":
			feature = "summary"
		case "translate":
			feature = "translation"
		case "chat":
			feature = "chat"
		case "explain":
			feature = "explanation"
		}
	}
	if len(parts) == 4 && parts[0] == "search" && parts[1] == "chats" && parts[3] == "messages" && canonicalUUID(parts[2]) != "" {
		feature = "search"
	}
	if feature == "" {
		return nil
	}
	id := canonicalUUID(r.Header.Get(ActionHeader))
	if id == "" {
		id = uuid.NewString()
	}
	locale := r.Header.Get("X-Research-Locale")
	if locale != "ru" && locale != "en" {
		locale = "unknown"
	}
	device := r.Header.Get("X-Research-Device")
	if device != "mobile" && device != "desktop" {
		device = "unknown"
	}
	return &Operation{ID: id, SessionID: canonicalUUID(r.Header.Get(SessionHeader)), PaperID: paper, Feature: feature, Locale: locale, Device: device, StartedAt: time.Now()}
}

// Gateway labels are deliberately coarse; domain services use the registered chi route.
func routeLabel(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
		return rc.RoutePattern()
	}
	parts := strings.Split(r.URL.Path, "/")
	for i, part := range parts {
		if canonicalUUID(part) != "" {
			parts[i] = "{id}"
		}
	}
	path := strings.Join(parts, "/")
	if knownRoutes[path] {
		return path
	}
	return "unmatched"
}

func HTTP(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var op *Operation
		if service != "gateway" {
			op = requestOperation(r)
		}
		if op != nil {
			r = r.WithContext(context.WithValue(r.Context(), operationKey{}, op))
			w.Header().Set(ActionHeader, op.ID)
		}
		tw := &responseWriter{ResponseWriter: w, op: op}
		var out http.ResponseWriter = tw
		if _, ok := w.(http.Flusher); ok {
			out = &flushingWriter{tw}
		}
		HTTPInflight.WithLabelValues(service).Inc()
		defer func() {
			HTTPInflight.WithLabelValues(service).Dec()
			panicked := recover()
			status := tw.status
			if status == 0 {
				status = 200
			}
			if panicked != nil {
				status = 500
			}
			method := r.Method
			switch method {
			case "GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS", "HEAD":
			default:
				method = "OTHER"
			}
			HTTPRequests.WithLabelValues(service, routeLabel(r), method, strconv.Itoa(status)).Inc()
			HTTPDuration.WithLabelValues(service, routeLabel(r), method).Observe(time.Since(started).Seconds())
			if op != nil {
				op.FinishedAt = time.Now()
				op.Outcome = "success"
				if status >= 400 || tw.streamError || panicked != nil {
					op.Outcome = "failed"
				}
				if op.Feature == "summary" && (status == 409 || status == 425) {
					op.Outcome = "waiting"
				}
				if tw.isStream && !tw.streamDone {
					op.Outcome = "failed"
				}
				if r.Context().Err() != nil && !tw.streamDone {
					op.Outcome = "cancelled"
				}
				cache := "miss"
				if op.CacheHit {
					cache = "hit"
				}
				ActionResults.WithLabelValues(service, op.Feature, op.Outcome, cache).Inc()
				ActionDuration.WithLabelValues(op.Feature, op.Outcome).Observe(op.FinishedAt.Sub(op.StartedAt).Seconds())
				if op.Feature == "summary" && op.Outcome == "success" && op.InputSource != "" {
					summarySources.WithLabelValues(op.InputSource, cache).Inc()
				}
				notify(op, "finished")
			}
			if panicked != nil {
				panic(panicked)
			}
		}()
		next.ServeHTTP(out, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	op                                                        *Operation
	status                                                    int
	isStream, streamDone, streamError, firstContent, overflow bool
	buffer                                                    []byte
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.isStream = strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream")
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	n, err := w.ResponseWriter.Write(data)
	if w.op != nil && !w.overflow {
		if w.isStream {
			w.feed(data[:n])
		} else if strings.Contains(w.Header().Get("Content-Type"), "application/json") && len(w.buffer)+n <= 1<<20 {
			w.buffer = append(w.buffer, data[:n]...)
			var body map[string]json.RawMessage
			if json.Unmarshal(w.buffer, &body) == nil {
				w.content(body)
			}
		}
	}
	return n, err
}
func (w *responseWriter) feed(data []byte) {
	w.buffer = append(w.buffer, data...)
	for {
		end := bytes.Index(w.buffer, []byte("\n\n"))
		separator := 2
		if crlf := bytes.Index(w.buffer, []byte("\r\n\r\n")); crlf >= 0 && (end < 0 || crlf < end) {
			end = crlf
			separator = 4
		}
		if end < 0 {
			break
		}
		frame := string(w.buffer[:end])
		w.buffer = w.buffer[end+separator:]
		name, raw := "", ""
		for _, line := range strings.Split(frame, "\n") {
			if strings.HasPrefix(line, "event:") {
				name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			}
			if strings.HasPrefix(line, "data:") {
				raw += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
		var body map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &body) != nil {
			continue
		}
		var kind string
		_ = json.Unmarshal(body["type"], &kind)
		if kind != "" {
			name = kind
		}
		switch name {
		case "error":
			w.streamError = true
		case "done":
			w.streamDone = true
			w.content(body)
		case "delta":
			var text, content string
			_ = json.Unmarshal(body["text"], &text)
			_ = json.Unmarshal(body["content"], &content)
			if strings.TrimSpace(text+content) != "" {
				w.observeFirst()
			}
		}
	}
	if len(w.buffer) > 1<<20 {
		w.buffer = nil
		w.overflow = true
		w.streamError = true
	}
}
func (w *responseWriter) observeFirst() {
	if !w.firstContent && w.op != nil {
		w.firstContent = true
		FirstContent.WithLabelValues(w.op.Feature).Observe(time.Since(w.op.StartedAt).Seconds())
	}
}
func (w *responseWriter) content(body map[string]json.RawMessage) {
	for _, field := range []string{"reply", "translation", "content"} {
		var value string
		_ = json.Unmarshal(body[field], &value)
		if strings.TrimSpace(value) != "" {
			w.observeFirst()
		}
	}
	var id string
	_ = json.Unmarshal(body["message_id"], &id)
	if id == "" {
		_ = json.Unmarshal(body["id"], &id)
	}
	if canonicalUUID(id) != "" {
		w.op.ResultID = canonicalUUID(id)
	}
	if len(body["summary"]) > 0 {
		w.observeFirst()
	}
}

type flushingWriter struct{ *responseWriter }

func (w *flushingWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	w.ResponseWriter.(http.Flusher).Flush()
}
