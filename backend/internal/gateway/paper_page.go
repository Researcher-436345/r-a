package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/google/uuid"
)

type paperPageHandler struct {
	frontendURL string
	catalogURL  string
	publicURL   string
	client      *http.Client
}

type paperMetadata struct {
	Title       string
	Description string
	URL         string
	ImageURL    string
}

var pageTitleRE = regexp.MustCompile(`(?s)<title>.*?</title>`)
var paperMetadataTemplate = template.Must(template.New("paper-metadata").Parse(`
<title>{{.Title}} · Odyssey</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.URL}}">
<meta property="og:type" content="article">
<meta property="og:site_name" content="Odyssey">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.URL}}">
<meta property="og:image" content="{{.ImageURL}}">
<meta property="og:image:alt" content="Odyssey — научные статьи">
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="{{.Title}}">
<meta name="twitter:description" content="{{.Description}}">
<meta name="twitter:image" content="{{.ImageURL}}">
`))

func (h paperPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := uuid.Parse(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/reader/"), "/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Serve the same application shell to browsers and preview crawlers.
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.frontendURL+"/index.html", nil)
	if err != nil {
		http.Error(w, "Application unavailable", http.StatusServiceUnavailable)
		return
	}
	response, err := h.client.Do(request)
	if err != nil {
		http.Error(w, "Application unavailable", http.StatusServiceUnavailable)
		return
	}
	defer response.Body.Close()

	const maxHTML = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxHTML+1))
	if err != nil || len(body) > maxHTML || response.StatusCode != http.StatusOK ||
		!strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
		http.Error(w, "Application unavailable", http.StatusServiceUnavailable)
		return
	}

	// The metadata lookup is deliberately anonymous: uploaded private papers
	// must never leak their title or authors through a public link preview.
	if paper, err := h.publicPaper(r, id); err == nil {
		var tags bytes.Buffer
		if err := paperMetadataTemplate.Execute(&tags, h.metadata(paper)); err == nil {
			body = pageTitleRE.ReplaceAll(body, nil)
			body = bytes.Replace(body, []byte("</head>"), append(tags.Bytes(), []byte("</head>")...), 1)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	for _, name := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}

func (h paperPageHandler) publicPaper(r *http.Request, id uuid.UUID) (catalog.PaperOut, error) {
	var paper catalog.PaperOut
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.catalogURL+"/papers/"+id.String(), nil)
	if err != nil {
		return paper, err
	}
	response, err := h.client.Do(request)
	if err != nil {
		return paper, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return paper, fmt.Errorf("public paper: HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&paper); err != nil {
		return paper, err
	}
	return paper, nil
}

func (h paperPageHandler) metadata(paper catalog.PaperOut) paperMetadata {
	var description []string
	for _, author := range paper.Authors[:min(3, len(paper.Authors))] {
		description = append(description, author.Name)
	}
	if len(paper.Authors) > 3 {
		description = append(description, "и др.")
	}
	if paper.Year != nil {
		description = append(description, fmt.Sprint(*paper.Year))
	}
	description = append(description, "Читайте статью в Odyssey")
	origin := strings.TrimRight(h.publicURL, "/")
	return paperMetadata{
		Title:       strings.Join(strings.Fields(paper.Title), " "),
		Description: strings.Join(description, " · "),
		URL:         origin + "/reader/" + paper.ID.String(),
		ImageURL:    origin + "/og-image.png",
	}
}
