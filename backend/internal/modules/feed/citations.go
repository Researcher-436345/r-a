package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	citationCacheTTL      = 24 * time.Hour
	citationMissCacheTTL  = 6 * time.Hour
	openAlexHTTPTimeout   = 12 * time.Second
	semanticScholarTimeout = 12 * time.Second
)

// CitationConfig — внешние источники цитирований.
// Сейчас работает OpenAlex без ключа (mailto).
// Semantic Scholar включается, когда появится SEMANTIC_SCHOLAR_API_KEY.
type CitationConfig struct {
	Enabled               bool
	OpenAlexMailto        string
	SemanticScholarAPIKey string
}

type citationRecord struct {
	Count  int    `json:"count"`
	Found  bool   `json:"found"`
	Source string `json:"source"`
}

type openAlexWork struct {
	CitedByCount int `json:"cited_by_count"`
	IDs          struct {
		DOI string `json:"doi"`
	} `json:"ids"`
}

type openAlexListResponse struct {
	Results []openAlexWork `json:"results"`
}

type semanticScholarPaper struct {
	CitationCount *int `json:"citationCount"`
	ExternalIDs   *struct {
		ArXiv string `json:"ArXiv"`
	} `json:"externalIds"`
}

func (f Service) enrichCitations(ctx context.Context, items []TrendingPaper) []TrendingPaper {
	if !f.Citations.Enabled || len(items) == 0 {
		return items
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ArxivID)
	}

	lookup := f.loadCitationCache(ctx, ids)
	missing := make([]string, 0)
	for _, id := range ids {
		if _, ok := lookup[id]; !ok {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		fetched := f.fetchCitations(ctx, missing)
		for id, rec := range fetched {
			lookup[id] = rec
			f.storeCitationCache(ctx, id, rec)
		}
		// Пометить ненайденные, чтобы не долбить API на каждом запросе ленты.
		for _, id := range missing {
			if _, ok := lookup[id]; ok {
				continue
			}
			miss := citationRecord{Found: false, Source: "none"}
			lookup[id] = miss
			f.storeCitationCache(ctx, id, miss)
		}
	}

	for i := range items {
		rec, ok := lookup[items[i].ArxivID]
		if !ok || !rec.Found {
			continue
		}
		count := rec.Count
		source := rec.Source
		items[i].CitationCount = &count
		items[i].CitationSource = &source
	}
	return items
}

func (f Service) fetchCitations(ctx context.Context, arxivIDs []string) map[string]citationRecord {
	out := make(map[string]citationRecord, len(arxivIDs))

	// Prefer Semantic Scholar when key is set (лучше lookup по ARXIV:id).
	if strings.TrimSpace(f.Citations.SemanticScholarAPIKey) != "" {
		for id, rec := range f.fetchSemanticScholar(ctx, arxivIDs) {
			out[id] = rec
		}
	}

	stillMissing := make([]string, 0)
	for _, id := range arxivIDs {
		if _, ok := out[id]; !ok {
			stillMissing = append(stillMissing, id)
		}
	}
	if len(stillMissing) == 0 {
		return out
	}

	for id, rec := range f.fetchOpenAlex(ctx, stillMissing) {
		out[id] = rec
	}
	return out
}

func (f Service) fetchOpenAlex(ctx context.Context, arxivIDs []string) map[string]citationRecord {
	out := make(map[string]citationRecord, len(arxivIDs))
	if len(arxivIDs) == 0 {
		return out
	}

	dois := make([]string, 0, len(arxivIDs))
	for _, id := range arxivIDs {
		dois = append(dois, "https://doi.org/10.48550/arXiv."+id)
	}
	filter := "doi:" + strings.Join(dois, "|")
	mailto := strings.TrimSpace(f.Citations.OpenAlexMailto)
	if mailto == "" {
		mailto = "researcher@localhost"
	}

	endpoint := "https://api.openalex.org/works?filter=" + url.QueryEscape(filter) +
		"&select=cited_by_count,ids&per_page=50&mailto=" + url.QueryEscape(mailto)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out
	}
	req.Header.Set("User-Agent", "researcher-api/0.1 (mailto:"+mailto+")")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: openAlexHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return out
	}

	var payload openAlexListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return out
	}

	for _, work := range payload.Results {
		id := arxivIDFromDOI(work.IDs.DOI)
		if id == "" {
			continue
		}
		out[id] = citationRecord{
			Count:  work.CitedByCount,
			Found:  true,
			Source: "openalex",
		}
	}
	return out
}

func (f Service) fetchSemanticScholar(ctx context.Context, arxivIDs []string) map[string]citationRecord {
	out := make(map[string]citationRecord, len(arxivIDs))
	if len(arxivIDs) == 0 {
		return out
	}

	ids := make([]string, 0, len(arxivIDs))
	for _, id := range arxivIDs {
		ids = append(ids, "ARXIV:"+id)
	}
	body, err := json.Marshal(map[string]any{"ids": ids})
	if err != nil {
		return out
	}
	endpoint := "https://api.semanticscholar.org/graph/v1/paper/batch?fields=citationCount,externalIds"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return out
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", f.Citations.SemanticScholarAPIKey)

	client := &http.Client{Timeout: semanticScholarTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return out
	}

	var papers []*semanticScholarPaper
	if err := json.NewDecoder(resp.Body).Decode(&papers); err != nil {
		return out
	}
	for i, paper := range papers {
		if paper == nil || paper.CitationCount == nil {
			continue
		}
		id := ""
		if paper.ExternalIDs != nil && paper.ExternalIDs.ArXiv != "" {
			id = canonicalArxivID(paper.ExternalIDs.ArXiv)
		}
		if id == "" && i < len(arxivIDs) {
			id = arxivIDs[i]
		}
		if id == "" {
			continue
		}
		out[id] = citationRecord{
			Count:  *paper.CitationCount,
			Found:  true,
			Source: "semanticscholar",
		}
	}
	return out
}

func (f Service) loadCitationCache(ctx context.Context, ids []string) map[string]citationRecord {
	out := make(map[string]citationRecord, len(ids))
	if f.Redis == nil || len(ids) == 0 {
		return out
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, citationCacheKey(id))
	}
	values, err := f.Redis.MGet(ctx, keys...).Result()
	if err != nil {
		return out
	}
	for i, raw := range values {
		s, ok := raw.(string)
		if !ok || s == "" {
			continue
		}
		var rec citationRecord
		if json.Unmarshal([]byte(s), &rec) != nil {
			continue
		}
		out[ids[i]] = rec
	}
	return out
}

func (f Service) storeCitationCache(ctx context.Context, arxivID string, rec citationRecord) {
	if f.Redis == nil {
		return
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	ttl := citationCacheTTL
	if !rec.Found {
		ttl = citationMissCacheTTL
	}
	_ = f.Redis.Set(ctx, citationCacheKey(arxivID), b, ttl).Err()
}

func citationCacheKey(arxivID string) string {
	return fmt.Sprintf("cite:v1:%s", strings.ToLower(arxivID))
}

func arxivIDFromDOI(doi string) string {
	doi = strings.TrimSpace(doi)
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	lower := strings.ToLower(doi)
	const prefix = "10.48550/arxiv."
	if !strings.HasPrefix(lower, prefix) {
		return ""
	}
	return canonicalArxivID(doi[len(prefix):])
}
