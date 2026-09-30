package client

// Annotation mirrors the annotation JSON.
type Annotation struct {
	ID                  string         `json:"id"`
	PaperID             string         `json:"paper_id"`
	Page                int            `json:"page"`
	Rect                map[string]any `json:"rect"`
	SelectedText        string         `json:"selected_text"`
	Note                string         `json:"note"`
	Color               string         `json:"color"`
	SourceChatMessageID *string        `json:"source_chat_message_id"`
}

// Annotations lists the user's notes on a paper.
func (s *Session) Annotations(paperID string) ([]Annotation, Response) {
	s.t.Helper()
	resp := s.Get("/papers/" + paperID + "/annotations")
	var out []Annotation
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out, resp
}

// CreateAnnotation saves a note on a selection; body is the raw request.
func (s *Session) CreateAnnotation(paperID string, body map[string]any) (Annotation, Response) {
	s.t.Helper()
	resp := s.Post("/papers/"+paperID+"/annotations", body)
	var a Annotation
	if resp.Status == 201 {
		resp.JSON(s.t, &a)
	}
	return a, resp
}

// Note is the common case: a note on selected text on a page.
func (s *Session) Note(paperID string, page int, selected, note string) Annotation {
	s.t.Helper()
	a, resp := s.CreateAnnotation(paperID, map[string]any{"page": page, "selected_text": selected, "note": note})
	if resp.Status != 201 {
		s.t.Fatalf("create annotation: %s", resp)
	}
	return a
}

// PatchAnnotation changes the note text.
func (s *Session) PatchAnnotation(id string, body map[string]any) (Annotation, Response) {
	s.t.Helper()
	resp := s.Patch("/annotations/"+id, body)
	var a Annotation
	if resp.Status == 200 {
		resp.JSON(s.t, &a)
	}
	return a, resp
}

// DeleteAnnotation removes a note.
func (s *Session) DeleteAnnotation(id string) Response {
	s.t.Helper()
	return s.Delete("/annotations/" + id)
}
