package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var crossrefBaseURL = "https://api.crossref.org"

type CrossrefPaper struct {
	DOI, Title      string
	Type            string
	Abstract, Venue *string
	Authors         []string
	Year            *int
}

// crossrefContainerTypes name DOIs of whole volumes, series and journals
// (e.g. 10.52202/075280 is all of NeurIPS 36) and fragments of works. None
// of them is a paper a reader can open.
var crossrefContainerTypes = map[string]bool{
	"proceedings":        true,
	"proceedings-series": true,
	"book-series":        true,
	"book-set":           true,
	"journal":            true,
	"journal-volume":     true,
	"journal-issue":      true,
	"report-series":      true,
	"component":          true,
	"grant":              true,
}

type crossrefDate struct {
	DateParts [][]int `json:"date-parts"`
}

func isContainerCrossrefType(t string) bool {
	return crossrefContainerTypes[strings.ToLower(strings.TrimSpace(t))]
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

// lookupDOIByPII maps an Elsevier PII (the id in sciencedirect.com URLs) to
// its DOI; Elsevier deposits PIIs as Crossref alternative ids.
func lookupDOIByPII(ctx context.Context, pii string) (string, error) {
	endpoint := crossrefBaseURL + "/works?rows=2&select=DOI&filter=alternative-id:" + url.QueryEscape(strings.ToUpper(pii))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "researcher-mvp/0.1 (mailto:dev@local)")
	resp, err := articleHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("Crossref returned %s", resp.Status)
	}
	var data struct {
		Message struct {
			Items []struct {
				DOI string `json:"DOI"`
			} `json:"items"`
		} `json:"message"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); err != nil {
		return "", err
	}
	// Two works under one PII would be a guess.
	if len(data.Message.Items) != 1 {
		return "", fmt.Errorf("%w: PII %s", errUnsupportedArticleURL, pii)
	}
	return NormalizeDOI(data.Message.Items[0].DOI)
}

func FetchCrossrefMetadata(ctx context.Context, doi string) (CrossrefPaper, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crossrefBaseURL+"/works/"+url.PathEscape(doi), nil)
	if err != nil {
		return CrossrefPaper{}, err
	}
	req.Header.Set("User-Agent", "researcher-mvp/0.1 (mailto:dev@local)")
	resp, err := articleHTTPClient.Do(req)
	if err != nil {
		return CrossrefPaper{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return CrossrefPaper{}, fmt.Errorf("%w: DOI not found", errUnsupportedArticleURL)
	}
	if resp.StatusCode/100 != 2 {
		return CrossrefPaper{}, fmt.Errorf("Crossref returned %s", resp.Status)
	}
	var data struct {
		Message struct {
			Type     string   `json:"type"`
			Title    []string `json:"title"`
			Abstract string   `json:"abstract"`
			Author   []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
			} `json:"author"`
			Container       []string     `json:"container-title"`
			Issued          crossrefDate `json:"issued"`
			PublishedPrint  crossrefDate `json:"published-print"`
			PublishedOnline crossrefDate `json:"published-online"`
			Created         crossrefDate `json:"created"`
		}
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&data); err != nil {
		return CrossrefPaper{}, err
	}
	p := CrossrefPaper{DOI: doi, Type: strings.ToLower(strings.TrimSpace(data.Message.Type))}
	if len(data.Message.Title) > 0 && strings.TrimSpace(data.Message.Title[0]) != "" {
		p.Title = cleanMetadataValue(htmlTagRE.ReplaceAllString(data.Message.Title[0], ""))
	} else {
		p.Title = "DOI:" + doi
	}
	if data.Message.Abstract != "" {
		s := strings.Join(strings.Fields(htmlTagRE.ReplaceAllString(data.Message.Abstract, " ")), " ")
		s = strings.TrimSpace(abstractLabelRE.ReplaceAllString(s, ""))
		if s != "" {
			p.Abstract = &s
		}
	}
	if len(data.Message.Container) > 0 {
		p.Venue = &data.Message.Container[0]
	}
	for _, a := range data.Message.Author {
		if n := strings.TrimSpace(a.Given + " " + a.Family); n != "" {
			p.Authors = append(p.Authors, n)
		}
	}
	// "created" is when the DOI was registered, so it is only the last resort.
	for _, d := range [][][]int{data.Message.Issued.DateParts, data.Message.PublishedPrint.DateParts, data.Message.PublishedOnline.DateParts, data.Message.Created.DateParts} {
		if len(d) > 0 && len(d[0]) > 0 && d[0][0] > 0 {
			y := d[0][0]
			p.Year = &y
			break
		}
	}
	return p, nil
}
