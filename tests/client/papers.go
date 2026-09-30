package client

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

// Paper mirrors the fields of GET /papers/{id} the scenarios care about.
type Paper struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Abstract *string `json:"abstract"`
	ArxivID  *string `json:"arxiv_id"`
	DOI      *string `json:"doi"`
	Authors  []struct {
		Name string `json:"name"`
	} `json:"authors"`
	LatestVersion *struct {
		ID           string  `json:"id"`
		Source       string  `json:"source"`
		Status       string  `json:"status"`
		PDFKey       *string `json:"pdf_key"`
		ErrorMessage *string `json:"error_message"`
	} `json:"latest_version"`
	HasFullText bool `json:"has_full_text"`
}

// OpenArxiv is what clicking a feed card does (public, never touches a library).
func (s *Session) OpenArxiv(arxivID string) (Paper, Response) {
	s.t.Helper()
	resp := s.Post("/papers/arxiv/open", map[string]any{"arxiv_id": arxivID})
	var p Paper
	if resp.Status == 201 || resp.Status == 200 {
		resp.JSON(s.t, &p)
	}
	return p, resp
}

// AddArxiv is the "arXiv" tab of the add-paper page.
func (s *Session) AddArxiv(arxivID string) (Paper, Response) {
	s.t.Helper()
	resp := s.Post("/papers/arxiv", map[string]any{"arxiv_id": arxivID})
	var p Paper
	if resp.Status == 201 || resp.Status == 200 {
		resp.JSON(s.t, &p)
	}
	return p, resp
}

// UploadPDF is the "PDF file" tab of the add-paper page.
func (s *Session) UploadPDF(filename string, data []byte) (Paper, Response) {
	s.t.Helper()
	resp := s.PostMultipart("/papers/upload", "file", filename, "application/pdf", data)
	var p Paper
	if resp.Status == 201 || resp.Status == 200 {
		resp.JSON(s.t, &p)
	}
	return p, resp
}

// GetPaper reads the paper card.
func (s *Session) GetPaper(id string) (Paper, Response) {
	s.t.Helper()
	resp := s.Get("/papers/" + id)
	var p Paper
	if resp.Status == 200 {
		resp.JSON(s.t, &p)
	}
	return p, resp
}

// MustGetPaper fails the test unless the card is readable.
func (s *Session) MustGetPaper(id string) Paper {
	s.t.Helper()
	p, resp := s.GetPaper(id)
	if resp.Status != 200 {
		s.t.Fatalf("GET /papers/%s: %s", id, resp)
	}
	return p
}

// WaitForPdfReady polls /pdf-url like the reader does: 409 pdf_processing
// means keep waiting; anything else ends the wait and is returned.
func (s *Session) WaitForPdfReady(id string, timeout time.Duration) Response {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	var last Response
	for {
		last = s.Get("/papers/" + id + "/pdf-url")
		if !(last.Status == http.StatusConflict && last.Code() == "pdf_processing") {
			return last
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("PDF of %s still processing after %s: %s", id, timeout, last)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// WaitForVersionStatus polls the paper card until latest_version.status matches.
func (s *Session) WaitForVersionStatus(id, want string, timeout time.Duration) Paper {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		p := s.MustGetPaper(id)
		if p.LatestVersion != nil && p.LatestVersion.Status == want {
			return p
		}
		if time.Now().After(deadline) {
			status := "<none>"
			if p.LatestVersion != nil {
				status = fmt.Sprintf("%s (%v)", p.LatestVersion.Status, p.LatestVersion.ErrorMessage)
			}
			s.t.Fatalf("paper %s: wanted version status %q, still %s after %s", id, want, status, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// WaitForFullText polls until the parsed text is stored.
func (s *Session) WaitForFullText(id string, timeout time.Duration) Paper {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		p := s.MustGetPaper(id)
		if p.HasFullText {
			return p
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("paper %s: no full text after %s", id, timeout)
		}
		time.Sleep(time.Second)
	}
}

// PDF downloads the stored file.
func (s *Session) PDF(id string) Response {
	s.t.Helper()
	return s.Get("/papers/" + id + "/pdf")
}

// Translate is the selection popup's translation request (non-streaming).
func (s *Session) Translate(paperID, text, lang string) Response {
	s.t.Helper()
	return s.Post("/papers/"+paperID+"/translate", map[string]string{"text": text, "target_lang": lang})
}

// TranslateStream is the instant-translation request the reader makes.
func (s *Session) TranslateStream(paperID, text, lang string) []Event {
	s.t.Helper()
	resp := s.Stream(http.MethodPost, "/papers/"+paperID+"/translate?stream=1", map[string]string{"text": text, "target_lang": lang})
	return ReadSSE(s.t, resp)
}

// LibraryItem is one row of GET /library.
type LibraryItem struct {
	ID       string  `json:"id"`
	Status   string  `json:"status"`
	Favorite bool    `json:"favorite"`
	FolderID *string `json:"folder_id"`
	Paper    Paper   `json:"paper"`
}

// Library lists the user's papers (first page, up to 100).
func (s *Session) Library() ([]LibraryItem, Response) {
	s.t.Helper()
	resp := s.Get("/library?limit=100")
	var out struct {
		Items []LibraryItem `json:"items"`
	}
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out.Items, resp
}

// HasInLibrary reports whether the paper is in the user's library.
func (s *Session) HasInLibrary(paperID string) bool {
	s.t.Helper()
	items, resp := s.Library()
	if resp.Status != 200 {
		s.t.Fatalf("GET /library: %s", resp)
	}
	for _, it := range items {
		if it.Paper.ID == paperID {
			return true
		}
	}
	return false
}

// AddDOI is the "DOI" tab of the add-paper page.
func (s *Session) AddDOI(doi string) (Paper, Response) {
	s.t.Helper()
	resp := s.Post("/papers/doi", map[string]any{"doi": doi})
	var p Paper
	if resp.Status == 201 || resp.Status == 200 {
		resp.JSON(s.t, &p)
	}
	return p, resp
}

// AddFromURL is the "link" tab (and what the search result's "open" does).
func (s *Session) AddFromURL(url, titleHint string) (Paper, Response) {
	s.t.Helper()
	body := map[string]any{"url": url}
	if titleHint != "" {
		body["title_hint"] = titleHint
	}
	resp := s.Post("/papers/from-url", body)
	var p Paper
	if resp.Status == 201 || resp.Status == 200 {
		resp.JSON(s.t, &p)
	}
	return p, resp
}

// RetryPDF is the reader's "retry" button on a failed download.
func (s *Session) RetryPDF(id string) (Paper, Response) {
	s.t.Helper()
	resp := s.Post("/papers/"+id+"/retry-pdf", nil)
	var p Paper
	if resp.Status == 200 {
		resp.JSON(s.t, &p)
	}
	return p, resp
}

// FindFullText is the reader's "Find full text" button.
func (s *Session) FindFullText(id string) (Paper, Response) {
	s.t.Helper()
	resp := s.Post("/papers/"+id+"/find-fulltext", nil)
	var p Paper
	if resp.Status == 200 {
		resp.JSON(s.t, &p)
	}
	return p, resp
}

// ReadyArxiv opens an arXiv paper and waits until its PDF is stored.
func (s *Session) ReadyArxiv(arxivID string) Paper {
	s.t.Helper()
	p, resp := s.OpenArxiv(arxivID)
	if resp.Status != 201 && resp.Status != 200 {
		s.t.Fatalf("open %s: %s", arxivID, resp)
	}
	if ready := s.WaitForPdfReady(p.ID, 90*time.Second); ready.Status != 200 {
		s.t.Fatalf("pdf of %s: %s", arxivID, ready)
	}
	return p
}

// ParsedArxiv opens an arXiv paper and waits until its full text is parsed.
func (s *Session) ParsedArxiv(arxivID string) Paper {
	s.t.Helper()
	p := s.ReadyArxiv(arxivID)
	return s.WaitForFullText(p.ID, 3*time.Minute)
}

// Synthetic arXiv ids the fake serves for any 5-digit suffix; a scenario
// mints one per run so a persistent stand never hands it an old record.
const (
	SyntheticArxivOK          = "2350."
	SyntheticArxivUnavailable = "2351."
	SyntheticArxivNotPDF      = "2352."
)

// NewArxivID mints a fresh synthetic arXiv id with the given prefix.
func NewArxivID(prefix string) string {
	return prefix + fmt.Sprintf("%05d", rand.IntN(100000))
}

// Synthetic DOIs the fake Crossref/OpenAlex serve for any 5-digit suffix.
const (
	SyntheticDOIClosed = "10.5555/closed."
	SyntheticDOIOpen   = "10.5555/oa."
)

// NewDOI mints a fresh synthetic DOI with the given prefix.
func NewDOI(prefix string) string {
	return prefix + fmt.Sprintf("%05d", rand.IntN(100000))
}
