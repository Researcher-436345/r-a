package services

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var arxivRE = regexp.MustCompile(`(?i)(?:https?://(?:www\.)?arxiv\.org/(?:abs|pdf)/)?(\d{4}\.\d{4,5}(?:v\d+)?|[a-z-]+(?:\.[A-Z]{2})?/\d{7}(?:v\d+)?)`)

type ArxivPaper struct {
	ArxivID, Title, Abstract, PDFURL string
	Authors                          []string
	Year                             *int
}
type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}
type atomEntry struct {
	Title     string `xml:"title"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"`
	Authors   []struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Links []struct {
		Href  string `xml:"href,attr"`
		Type  string `xml:"type,attr"`
		Title string `xml:"title,attr"`
	} `xml:"link"`
}

func NormalizeArxivID(value string) (string, error) {
	m := arxivRE.FindStringSubmatch(strings.TrimSpace(value))
	if len(m) < 2 {
		return "", fmt.Errorf("Invalid arXiv id or URL")
	}
	return m[1], nil
}
func CanonicalArxivID(id string) string {
	return regexp.MustCompile(`(?i)v\d+$`).ReplaceAllString(id, "")
}
func FetchArxivMetadata(ctx context.Context, id string) (ArxivPaper, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://export.arxiv.org/api/query?id_list="+id, nil)
	if err != nil {
		return ArxivPaper{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ArxivPaper{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return ArxivPaper{}, fmt.Errorf("arXiv returned %s", resp.Status)
	}
	var feed atomFeed
	if err = xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return ArxivPaper{}, err
	}
	if len(feed.Entries) == 0 {
		return ArxivPaper{}, fmt.Errorf("arXiv paper not found")
	}
	e := feed.Entries[0]
	p := ArxivPaper{ArxivID: CanonicalArxivID(id), Title: strings.Join(strings.Fields(e.Title), " "), Abstract: strings.Join(strings.Fields(e.Summary), " "), PDFURL: "https://arxiv.org/pdf/" + CanonicalArxivID(id) + ".pdf"}
	if p.Title == "" {
		p.Title = "arXiv:" + p.ArxivID
	}
	if len(e.Published) >= 4 {
		var y int
		if _, err = fmt.Sscanf(e.Published[:4], "%d", &y); err == nil {
			p.Year = &y
		}
	}
	for _, a := range e.Authors {
		if n := strings.Join(strings.Fields(a.Name), " "); n != "" {
			p.Authors = append(p.Authors, n)
		}
	}
	for _, l := range e.Links {
		if l.Title == "pdf" || l.Type == "application/pdf" {
			p.PDFURL = l.Href
			break
		}
	}
	return p, nil
}
func DownloadPDF(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("PDF download returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 51<<20))
}
