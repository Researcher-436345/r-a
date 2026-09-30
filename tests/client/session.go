package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

// Session is one browser tab: a cookie jar (refresh cookie) plus the access
// token held in memory, exactly like the frontend.
type Session struct {
	t      testing.TB
	stand  Stand
	http   *http.Client
	Token  string
	Email  string
	Header http.Header // extra headers sent with every request (tests of X-User-Id etc.)
}

func newSession(t testing.TB, s Stand) *Session {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &Session{t: t, stand: s, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Header: http.Header{}}
}

// Response is a decoded gateway reply.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// JSON decodes the body into v; fails the test on malformed JSON.
func (r Response) JSON(t testing.TB, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode JSON (status %d): %v\nbody: %s", r.Status, err, r.Body)
	}
}

// Map decodes the body into a generic map.
func (r Response) Map(t testing.TB) map[string]any {
	t.Helper()
	m := map[string]any{}
	r.JSON(t, &m)
	return m
}

// Detail returns the error message the frontend would show ("detail").
func (r Response) Detail() string {
	var e struct {
		Detail any `json:"detail"`
	}
	_ = json.Unmarshal(r.Body, &e)
	return fmt.Sprint(e.Detail)
}

// Code returns the machine-readable error code, if any.
func (r Response) Code() string {
	var e struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(r.Body, &e)
	return e.Code
}

func (r Response) String() string {
	b := r.Body
	if len(b) > 600 {
		b = append(b[:600:600], []byte("…")...)
	}
	return fmt.Sprintf("HTTP %d: %s", r.Status, strings.TrimSpace(string(b)))
}

// Do sends a request through the gateway. body may be nil, []byte, string or
// any JSON-encodable value. The access token is attached when present.
func (s *Session) Do(method, path string, body any, headers ...string) Response {
	s.t.Helper()
	var reader io.Reader
	contentType := ""
	switch b := body.(type) {
	case nil:
	case []byte:
		reader = bytes.NewReader(b)
		contentType = "application/json"
	case string:
		reader = strings.NewReader(b)
		contentType = "application/json"
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			s.t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(raw)
		contentType = "application/json"
	}
	req, err := http.NewRequest(method, s.stand.GatewayURL+path, reader)
	if err != nil {
		s.t.Fatalf("build request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if s.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	for k, v := range s.Header {
		req.Header[k] = v
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := s.http.Do(req)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		s.t.Fatalf("%s %s: read body: %v", method, path, err)
	}
	return Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
}

func (s *Session) Get(path string, headers ...string) Response {
	s.t.Helper()
	return s.Do(http.MethodGet, path, nil, headers...)
}
func (s *Session) Post(path string, body any, headers ...string) Response {
	s.t.Helper()
	return s.Do(http.MethodPost, path, body, headers...)
}
func (s *Session) Patch(path string, body any) Response {
	s.t.Helper()
	return s.Do(http.MethodPatch, path, body)
}
func (s *Session) Delete(path string) Response {
	s.t.Helper()
	return s.Do(http.MethodDelete, path, nil)
}

// PostMultipart uploads a file the way the "PDF file" tab does.
func (s *Session) PostMultipart(path, field, filename, contentType string, data []byte) Response {
	s.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, filename)}
	if contentType != "" {
		hdr["Content-Type"] = []string{contentType}
	}
	part, err := mw.CreatePart(hdr)
	if err != nil {
		s.t.Fatalf("multipart: %v", err)
	}
	_, _ = part.Write(data)
	_ = mw.Close()
	req, err := http.NewRequest(http.MethodPost, s.stand.GatewayURL+path, &buf)
	if err != nil {
		s.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if s.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		s.t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
}

// Stream sends a request and returns the raw response for SSE reading.
func (s *Session) Stream(method, path string, body any, headers ...string) *http.Response {
	s.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			s.t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, s.stand.GatewayURL+path, reader)
	if err != nil {
		s.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if s.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := s.http.Do(req)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// RefreshCookie returns the current refresh cookie value ("" when absent).
func (s *Session) RefreshCookie() string {
	u, _ := url.Parse(s.stand.GatewayURL)
	for _, c := range s.http.Jar.Cookies(u) {
		if c.Name == "researcher_refresh" {
			return c.Value
		}
	}
	return ""
}

// SetRefreshCookie replaces the refresh cookie (used to replay an old one).
func (s *Session) SetRefreshCookie(value string) {
	u, _ := url.Parse(s.stand.GatewayURL)
	s.http.Jar.SetCookies(u, []*http.Cookie{{Name: "researcher_refresh", Value: value, Path: "/"}})
}

// Clone makes a second "device": same stand, fresh cookies, no token.
func (s *Session) Clone() *Session {
	c := newSession(s.t, s.stand)
	c.Email = s.Email
	return c
}

// readResponse drains a raw response into a Response (for streams that
// answered with a plain error instead of SSE).
func readResponse(t testing.TB, resp *http.Response) Response {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
}

// withToken copies the access token onto a cloned session (a second tab of
// the same signed-in user, without another login).
func (s *Session) WithToken(token string) *Session {
	s.Token = token
	return s
}
