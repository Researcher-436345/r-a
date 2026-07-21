package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type CrossrefPaper struct {
	DOI, Title      string
	Abstract, Venue *string
	Authors         []string
	Year            *int
}

func NormalizeDOI(v string) (string, error) {
	v = strings.TrimSpace(v)
	for _, p := range []string{"https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "http://dx.doi.org/", "doi:"} {
		if strings.HasPrefix(strings.ToLower(v), p) {
			v = strings.TrimSpace(v[len(p):])
			break
		}
	}
	if !strings.HasPrefix(strings.ToLower(v), "10.") {
		return "", fmt.Errorf("Invalid DOI")
	}
	return strings.ToLower(v), nil
}
func FetchCrossrefMetadata(ctx context.Context, doi string) (CrossrefPaper, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.crossref.org/works/"+url.PathEscape(doi), nil)
	if err != nil {
		return CrossrefPaper{}, err
	}
	req.Header.Set("User-Agent", "researcher-mvp/0.1 (mailto:dev@local)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CrossrefPaper{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return CrossrefPaper{}, fmt.Errorf("DOI not found")
	}
	if resp.StatusCode/100 != 2 {
		return CrossrefPaper{}, fmt.Errorf("Crossref returned %s", resp.Status)
	}
	var data struct {
		Message struct {
			Title    []string `json:"title"`
			Abstract string   `json:"abstract"`
			Author   []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
			} `json:"author"`
			Container                                []string `json:"container-title"`
			PublishedPrint, PublishedOnline, Created struct {
				DateParts [][]int `json:"date-parts"`
			}
		}
	}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return CrossrefPaper{}, err
	}
	p := CrossrefPaper{DOI: doi}
	if len(data.Message.Title) > 0 {
		p.Title = data.Message.Title[0]
	} else {
		p.Title = "DOI:" + doi
	}
	if data.Message.Abstract != "" {
		s := strings.Join(strings.Fields(strings.NewReplacer("<jats:p>", " ", "</jats:p>", " ").Replace(data.Message.Abstract)), " ")
		p.Abstract = &s
	}
	if len(data.Message.Container) > 0 {
		p.Venue = &data.Message.Container[0]
	}
	for _, a := range data.Message.Author {
		if n := strings.TrimSpace(a.Given + " " + a.Family); n != "" {
			p.Authors = append(p.Authors, n)
		}
	}
	for _, d := range [][][]int{data.Message.PublishedPrint.DateParts, data.Message.PublishedOnline.DateParts, data.Message.Created.DateParts} {
		if len(d) > 0 && len(d[0]) > 0 {
			y := d[0][0]
			p.Year = &y
			break
		}
	}
	return p, nil
}
