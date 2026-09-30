package client

import "net/http"

// Summary mirrors the paper overview JSON.
type Summary struct {
	PaperID   string  `json:"paper_id"`
	Lang      string  `json:"lang"`
	VersionID *string `json:"version_id"`
	Model     string  `json:"model"`
	Content   string  `json:"content"`
	Status    string  `json:"status"`
	Stale     bool    `json:"stale"`
}

// GetSummary reads the cached overview; query is e.g. "lang=en".
func (s *Session) GetSummary(paperID, query string) (Summary, Response) {
	s.t.Helper()
	path := "/papers/" + paperID + "/summary"
	if query != "" {
		path += "?" + query
	}
	resp := s.Get(path)
	var out Summary
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out, resp
}

// CreateSummary generates (or returns the cached) overview without streaming.
func (s *Session) CreateSummary(paperID, query string) (Summary, Response) {
	s.t.Helper()
	path := "/papers/" + paperID + "/summary"
	if query != "" {
		path += "?" + query
	}
	resp := s.Post(path, nil)
	var out Summary
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out, resp
}

// CreateSummaryStream is what the "Обзор" tab does: POST with stream=1.
// Non-stream error statuses (425, 409, 401…) are returned as the Response
// with a nil event list.
func (s *Session) CreateSummaryStream(paperID, query string) ([]Event, Response) {
	s.t.Helper()
	path := "/papers/" + paperID + "/summary?stream=1"
	if query != "" {
		path += "&" + query
	}
	resp := s.Stream(http.MethodPost, path, nil)
	if resp.StatusCode != 200 {
		return nil, readResponse(s.t, resp)
	}
	return ReadSSE(s.t, resp), Response{Status: 200, Header: resp.Header}
}
