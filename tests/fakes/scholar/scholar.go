// Package scholar is a stand-in for arXiv, OpenAlex, Crossref and Europe PMC.
// It answers with fixture data and can be told to fail on demand through
// /_control/*. The backend does not know it talks to a fake: the test network
// maps the real host names to this server (see docker-compose.test.yml).
package scholar

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/centraluniversity/researcher/tests/fixtures"
)

// Rule makes matching requests fail. Times==0 means "until reset".
type Rule struct {
	PathPrefix string `json:"path_prefix"`
	Status     int    `json:"status"`
	Body       string `json:"body,omitempty"`
	DelayMS    int    `json:"delay_ms,omitempty"`
	Times      int    `json:"times,omitempty"`
}

// Request is one recorded upstream call.
type Request struct {
	Method string    `json:"method"`
	Host   string    `json:"host"`
	Path   string    `json:"path"`
	Query  string    `json:"query"`
	At     time.Time `json:"at"`
}

type Server struct {
	mu       sync.Mutex
	rules    []Rule
	requests []Request
	caPEM    []byte
}

func New(caPEM []byte) *Server { return &Server{caPEM: caPEM} }

var versionRE = regexp.MustCompile(`(?i)v\d+$`)

func canonical(id string) string {
	id = strings.TrimSuffix(strings.TrimSpace(id), ".pdf")
	return versionRE.ReplaceAllString(id, "")
}

// Public serves the impersonated hosts (plain and TLS listeners share it).
func (s *Server) Public() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/query", s.arxivQuery)
	mux.HandleFunc("/pdf/", s.arxivPDF)
	mux.HandleFunc("/e-print/", s.arxivEPrint)
	mux.HandleFunc("/abs/", s.arxivAbs)
	mux.HandleFunc("/works", s.openAlexList)
	mux.HandleFunc("/works/", s.workByDOI)
	mux.HandleFunc("/oa/", s.openAccessPDF)
	mux.HandleFunc("/europepmc/", s.notFoundJSON)
	mux.HandleFunc("/graph/", s.notFoundJSON)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fake-scholar: unknown path "+r.URL.Path, http.StatusNotFound)
	})
	// OpenAlex is asked for /works/https://doi.org/{doi}: ServeMux would
	// "clean" the double slash with a redirect, so that route bypasses it.
	direct := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/works/") {
			s.workByDOI(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
	return s.record(s.applyRules(direct))
}

// Control serves health and the test-facing control API.
func (s *Server) Control() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "ok", "service": "fake-scholar", "papers": len(Catalog)})
	})
	mux.HandleFunc("/ca.crt", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(s.caPEM)
	})
	mux.HandleFunc("/_control/rules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var rule Rule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil || rule.PathPrefix == "" || rule.Status == 0 {
				http.Error(w, "rule needs path_prefix and status", 400)
				return
			}
			s.mu.Lock()
			s.rules = append(s.rules, rule)
			s.mu.Unlock()
			writeJSON(w, 201, rule)
		case http.MethodDelete:
			s.mu.Lock()
			s.rules = nil
			s.mu.Unlock()
			w.WriteHeader(204)
		case http.MethodGet:
			s.mu.Lock()
			defer s.mu.Unlock()
			writeJSON(w, 200, s.rules)
		default:
			w.WriteHeader(405)
		}
	})
	mux.HandleFunc("/_control/requests", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			prefix := r.URL.Query().Get("path_prefix")
			out := make([]Request, 0, len(s.requests))
			for _, rq := range s.requests {
				if prefix == "" || strings.HasPrefix(rq.Path, prefix) {
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
	})
	mux.HandleFunc("/_control/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"arxiv": Catalog, "doi": DOIs})
	})
	return mux
}

func (s *Server) record(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, Request{Method: r.Method, Host: r.Host, Path: r.URL.Path, Query: r.URL.RawQuery, At: time.Now()})
		if len(s.requests) > 1000 {
			s.requests = s.requests[len(s.requests)-1000:]
		}
		s.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) applyRules(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		var hit *Rule
		for i := range s.rules {
			if strings.HasPrefix(r.URL.Path, s.rules[i].PathPrefix) {
				hit = &s.rules[i]
				break
			}
		}
		var rule Rule
		if hit != nil {
			rule = *hit
			if hit.Times > 0 {
				hit.Times--
				if hit.Times == 0 {
					s.rules = removeRule(s.rules, hit)
				}
			}
		}
		s.mu.Unlock()
		if hit == nil {
			next.ServeHTTP(w, r)
			return
		}
		if rule.DelayMS > 0 {
			time.Sleep(time.Duration(rule.DelayMS) * time.Millisecond)
		}
		w.WriteHeader(rule.Status)
		_, _ = w.Write([]byte(rule.Body))
	})
}

func removeRule(rules []Rule, target *Rule) []Rule {
	out := rules[:0]
	for i := range rules {
		if &rules[i] != target {
			out = append(out, rules[i])
		}
	}
	return out
}

// --- arXiv ------------------------------------------------------------------

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	Entries []atomEntry `xml:"entry"`
}
type atomLink struct {
	Href  string `xml:"href,attr"`
	Rel   string `xml:"rel,attr,omitempty"`
	Type  string `xml:"type,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
}
type atomAuthor struct {
	Name string `xml:"name"`
}
type atomEntry struct {
	ID        string       `xml:"id"`
	Updated   string       `xml:"updated"`
	Published string       `xml:"published"`
	Title     string       `xml:"title"`
	Summary   string       `xml:"summary"`
	Authors   []atomAuthor `xml:"author"`
	Links     []atomLink   `xml:"link"`
}

func entryFor(p Paper) atomEntry {
	e := atomEntry{
		ID:        "https://arxiv.org/abs/" + p.ID + "v1",
		Updated:   p.Updated.Format(time.RFC3339),
		Published: p.Published.Format(time.RFC3339),
		Title:     p.Title,
		Summary:   p.Abstract,
		Links: []atomLink{
			{Href: "https://arxiv.org/abs/" + p.ID + "v1", Rel: "alternate", Type: "text/html"},
			{Href: "https://arxiv.org/pdf/" + p.ID + "v1", Rel: "related", Type: "application/pdf", Title: "pdf"},
		},
	}
	for _, a := range p.Authors {
		e.Authors = append(e.Authors, atomAuthor{Name: a})
	}
	return e
}

func (s *Server) arxivQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	feed := atomFeed{Title: "fake arXiv Query"}
	if ids := q.Get("id_list"); ids != "" {
		for _, id := range strings.Split(ids, ",") {
			if p, ok := Find(id); ok {
				feed.Entries = append(feed.Entries, entryFor(p))
			}
		}
	} else {
		max, _ := strconv.Atoi(q.Get("max_results"))
		if max <= 0 {
			max = 10
		}
		terms := titleTerms(q.Get("search_query"))
		for _, p := range Healthy() {
			if len(feed.Entries) >= max {
				break
			}
			if titleHasAll(p.Title, terms) {
				feed.Entries = append(feed.Entries, entryFor(p))
			}
		}
	}
	w.Header().Set("Content-Type", "application/atom+xml; charset=UTF-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(feed)
}

func (s *Server) arxivPDF(w http.ResponseWriter, r *http.Request) {
	id := canonical(strings.TrimPrefix(r.URL.Path, "/pdf/"))
	p, ok := Find(id)
	if !ok {
		http.Error(w, "No such paper", http.StatusNotFound)
		return
	}
	switch p.Behavior {
	case PDFUnavailable:
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	case PDFNotPDF:
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		_, _ = w.Write(fixtures.PDF(p.PDF))
	default:
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(200)
		_, _ = w.Write(fixtures.PDF(p.PDF))
	}
}

func (s *Server) arxivEPrint(w http.ResponseWriter, r *http.Request) {
	id := canonical(strings.TrimPrefix(r.URL.Path, "/e-print/"))
	p, ok := Find(id)
	if !ok || len(p.EPrint) == 0 {
		http.Error(w, "No source available", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-eprint-tar")
	w.WriteHeader(200)
	_, _ = w.Write(p.EPrint)
}

func (s *Server) arxivAbs(w http.ResponseWriter, r *http.Request) {
	id := canonical(strings.TrimPrefix(r.URL.Path, "/abs/"))
	p, ok := Find(id)
	if !ok {
		http.Error(w, "Article not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<html><head><title>%s</title><meta name=\"citation_title\" content=\"%s\"><meta name=\"citation_pdf_url\" content=\"https://arxiv.org/pdf/%s\"></head><body><h1>%s</h1></body></html>", p.Title, p.Title, p.ID, p.Title)
}

// titleTerms extracts the words of an arXiv "ti:word AND ti:word" query.
// A category query (the trending feed) has no ti: terms and matches everything.
func titleTerms(query string) []string {
	var terms []string
	for _, part := range strings.Fields(query) {
		if strings.HasPrefix(part, "ti:") {
			terms = append(terms, strings.ToLower(strings.Trim(strings.TrimPrefix(part, "ti:"), `"()`)))
		}
	}
	return terms
}

func titleHasAll(title string, terms []string) bool {
	lower := strings.ToLower(title)
	for _, t := range terms {
		if !strings.Contains(lower, t) {
			return false
		}
	}
	return true
}

// --- Crossref / OpenAlex ------------------------------------------------------

// workByDOI serves both /works/{doi} on api.crossref.org and
// /works/https://doi.org/{doi} on api.openalex.org; the host tells them apart.
func (s *Server) workByDOI(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.URL.Path, "/works/")
	if unescaped, err := url.PathUnescape(raw); err == nil {
		raw = unescaped
	}
	doi := raw
	for _, prefix := range []string{"https://doi.org/", "http://doi.org/", "https:/doi.org/", "http:/doi.org/"} {
		doi = strings.TrimPrefix(doi, prefix)
	}
	rec, ok := FindDOI(doi)
	if strings.Contains(r.Host, "openalex") {
		if !ok || rec.OpenAlex == nil {
			s.notFoundJSON(w, r)
			return
		}
		writeJSON(w, 200, openAlexJSON(rec))
		return
	}
	if !ok || !rec.Crossref {
		writeJSON(w, 404, map[string]any{"status": "error", "message-type": "route-not-found", "message": "Resource not found."})
		return
	}
	writeJSON(w, 200, crossrefJSON(rec))
}

func crossrefJSON(rec DOIRecord) map[string]any {
	authors := make([]map[string]string, 0, len(rec.Authors))
	for _, a := range rec.Authors {
		given, family := a, ""
		if i := strings.LastIndex(a, " "); i > 0 {
			given, family = a[:i], a[i+1:]
		}
		authors = append(authors, map[string]string{"given": given, "family": family})
	}
	msg := map[string]any{
		"DOI":    rec.DOI,
		"type":   rec.Type,
		"title":  []string{rec.Title},
		"author": authors,
		"issued": map[string]any{"date-parts": [][]int{{rec.Year}}},
	}
	if rec.Abstract != "" {
		msg["abstract"] = "<jats:p>" + rec.Abstract + "</jats:p>"
	}
	if rec.Venue != "" {
		msg["container-title"] = []string{rec.Venue}
	}
	return map[string]any{"status": "ok", "message-type": "work", "message": msg}
}

func openAlexJSON(rec DOIRecord) map[string]any {
	authorships := make([]map[string]any, 0, len(rec.Authors))
	for _, a := range rec.Authors {
		authorships = append(authorships, map[string]any{"author": map[string]string{"display_name": a}})
	}
	loc := map[string]any{
		"landing_page_url": rec.OpenAlex.LandingPage,
		"pdf_url":          rec.OpenAlex.PDFURL,
		"is_oa":            rec.OpenAlex.PDFURL != "" || rec.OpenAlex.LandingPage != "",
		"source":           map[string]string{"display_name": rec.Venue},
	}
	return map[string]any{
		"id":               rec.OpenAlex.ID,
		"title":            rec.Title,
		"display_name":     rec.Title,
		"type":             "article",
		"publication_year": rec.Year,
		"doi":              "https://doi.org/" + rec.DOI,
		"ids":              map[string]string{"doi": "https://doi.org/" + rec.DOI},
		"authorships":      authorships,
		"primary_location": loc,
		"best_oa_location": loc,
		"locations":        []any{loc},
	}
}

// openAccessPDF serves /oa/{fixture}.pdf: the publisher copy OpenAlex points at.
func (s *Server) openAccessPDF(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/oa/")
	if _, err := fixtures.FS.ReadFile("pdf/" + name); err != nil {
		http.Error(w, "No such file", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.WriteHeader(200)
	_, _ = w.Write(fixtures.PDF(name))
}

// --- OpenAlex ---------------------------------------------------------------

func (s *Server) openAlexList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"meta": map[string]any{"count": 0}, "results": []any{}})
}

func (s *Server) notFoundJSON(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 404, map[string]any{"error": "not found", "message": "fake-scholar has no such record"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
