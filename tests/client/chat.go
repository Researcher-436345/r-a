package client

import "net/http"

// ChatMessage mirrors one stored chat message.
type ChatMessage struct {
	ID          string  `json:"id"`
	Role        string  `json:"role"`
	Content     string  `json:"content"`
	ContextText *string `json:"context_text"`
}

// Chat sends a question in the reader's chat and waits for the whole reply.
func (s *Session) Chat(paperID string, body map[string]any) Response {
	s.t.Helper()
	return s.Post("/papers/"+paperID+"/chat", body)
}

// ChatStream sends a question with ?stream=1 and returns the events.
func (s *Session) ChatStream(paperID string, body map[string]any) []Event {
	s.t.Helper()
	resp := s.Stream(http.MethodPost, "/papers/"+paperID+"/chat?stream=1", body)
	return ReadSSE(s.t, resp)
}

// ChatMessages lists the user's chat history for a paper.
func (s *Session) ChatMessages(paperID string) ([]ChatMessage, Response) {
	s.t.Helper()
	resp := s.Get("/papers/" + paperID + "/chat/messages")
	var out struct {
		Items []ChatMessage `json:"items"`
	}
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out.Items, resp
}

// ChatContext is the context meter the composer shows.
func (s *Session) ChatContext(paperID, query string) Response {
	s.t.Helper()
	path := "/papers/" + paperID + "/chat/context"
	if query != "" {
		path += "?" + query
	}
	return s.Get(path)
}

// Explain is the selection popup's "explain" action.
func (s *Session) Explain(paperID string, body map[string]any) Response {
	s.t.Helper()
	return s.Post("/papers/"+paperID+"/explain", body)
}

// Models lists the models the picker offers.
func (s *Session) Models() Response {
	s.t.Helper()
	return s.Get("/assistant/models")
}
