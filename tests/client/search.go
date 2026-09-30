package client

import (
	"net/http"

	"github.com/google/uuid"
)

// SearchChat mirrors one research chat (messages only on GET by id).
type SearchChat struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Mode     string `json:"mode"`
	Messages []struct {
		ID      string `json:"id"`
		Role    string `json:"role"`
		Content string `json:"content"`
		Sources []struct {
			Title  string `json:"title"`
			URL    string `json:"url"`
			Domain string `json:"domain"`
		} `json:"sources"`
	} `json:"messages"`
}

// NewSearchChatID is what the frontend does: it mints the chat id itself.
func NewSearchChatID() string { return uuid.NewString() }

// SearchChats lists the user's research chats, newest first.
func (s *Session) SearchChats() ([]SearchChat, Response) {
	s.t.Helper()
	resp := s.Get("/search/chats")
	var out []SearchChat
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out, resp
}

// SearchChatByID reads one chat with its messages.
func (s *Session) SearchChatByID(id string) (SearchChat, Response) {
	s.t.Helper()
	resp := s.Get("/search/chats/" + id)
	var out SearchChat
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out, resp
}

// DeleteSearchChat removes a chat.
func (s *Session) DeleteSearchChat(id string) Response {
	s.t.Helper()
	return s.Delete("/search/chats/" + id)
}

// Ask sends a message into a research chat and returns the SSE events, or
// the non-stream error response with a nil event list.
func (s *Session) Ask(chatID, message, mode string) ([]Event, Response) {
	s.t.Helper()
	body := map[string]any{"message": message}
	if mode != "" {
		body["mode"] = mode
	}
	resp := s.Stream(http.MethodPost, "/search/chats/"+chatID+"/messages", body)
	if resp.StatusCode != 200 {
		return nil, readResponse(s.t, resp)
	}
	return ReadSSE(s.t, resp), Response{Status: 200, Header: resp.Header}
}

// Named returns the events with the given "event:" name.
func Named(events []Event, name string) []Event {
	var out []Event
	for _, e := range events {
		if e.Name == name {
			out = append(out, e)
		}
	}
	return out
}
