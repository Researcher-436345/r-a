package feed

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Service struct {
	Redis     *redis.Client
	Citations CitationConfig
}

type SortMode string

const (
	SortNew     SortMode = "new"
	SortHot     SortMode = "hot"
	SortPopular SortMode = "popular"
)

func ParseSortMode(raw string) SortMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(SortNew), "recent", "submitted":
		return SortNew
	case string(SortHot), "updated":
		return SortHot
	case string(SortPopular), "top", "rising":
		return SortPopular
	default:
		return ""
	}
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}
type atomEntry struct {
	Title     string `xml:"title"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
	Authors   []struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Links []struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

type rankedPaper struct {
	item      TrendingPaper
	published time.Time
	updated   time.Time
	score     float64
}

func (f Service) Trending(ctx context.Context, category string, limit int, mode SortMode) ([]TrendingPaper, bool, error) {
	if category == "" {
		category = "cs.AI"
	}
	if mode == "" {
		mode = SortNew
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	key := fmt.Sprintf("feed:trending:v3:%s:%s:%d", category, mode, limit)
	if f.Redis != nil {
		if cached, err := f.Redis.Get(ctx, key).Result(); err == nil {
			var items []TrendingPaper
			if json.Unmarshal([]byte(cached), &items) == nil {
				return items, true, nil
			}
		}
	}
	items, err := fetchTrending(ctx, category, limit, mode)
	if err != nil {
		return nil, false, err
	}
	items = f.enrichCitations(ctx, items)
	if mode == SortPopular {
		items = rankByCitations(items)
	}
	if f.Redis != nil {
		if b, e := json.Marshal(items); e == nil {
			_ = f.Redis.Set(ctx, key, b, time.Hour).Err()
		}
	}
	return items, false, nil
}

func rankByCitations(items []TrendingPaper) []TrendingPaper {
	sort.SliceStable(items, func(i, j int) bool {
		ci, cj := items[i].CitationCount, items[j].CitationCount
		switch {
		case ci != nil && cj != nil && *ci != *cj:
			return *ci > *cj
		case ci != nil && cj == nil:
			return true
		case ci == nil && cj != nil:
			return false
		default:
			return items[i].PopularityScore > items[j].PopularityScore
		}
	})
	return items
}

func roundScore(v float64) float64 {
	return math.Round(v*10) / 10
}

// newScore — чем новее сабмит, тем выше.
func newScore(published time.Time) float64 {
	if published.IsZero() {
		return 1
	}
	return roundScore(math.Max(1, 1000*math.Exp(-time.Since(published).Hours()/24/14)))
}

// hotScore — свежесть last-update (короткое окно).
func hotScore(updated time.Time) float64 {
	if updated.IsZero() {
		return 1
	}
	return roundScore(math.Max(1, 1000*math.Exp(-time.Since(updated).Hours()/24/5)))
}

// popularScore — «rising» без голосов: пик через несколько дней после сабмита,
// бонус за ревизию (updated заметно позже published).
func popularScore(published, updated time.Time) float64 {
	if published.IsZero() {
		return 1
	}
	ageDays := math.Max(0, time.Since(published).Hours()/24)
	// Похоже на HN без points: (age+const) / (age+gravity)^g
	rising := (ageDays + 0.8) / math.Pow(ageDays+2.2, 1.6)
	boost := 1.0
	if !updated.IsZero() && updated.After(published.Add(8*time.Hour)) {
		boost = 1.55
	}
	return roundScore(math.Max(1, rising*boost*4000))
}

func canonicalArxivID(id string) string {
	return regexp.MustCompile(`(?i)v\d+$`).ReplaceAllString(id, "")
}

func arxivSortBy(mode SortMode) string {
	switch mode {
	case SortHot, SortPopular:
		// lastUpdated даёт v2/пересмотренные — заметно отличается от чистого new.
		return "lastUpdatedDate"
	default:
		return "submittedDate"
	}
}

func fetchTrending(ctx context.Context, category string, limit int, mode SortMode) ([]TrendingPaper, error) {
	fetchLimit := limit
	if mode == SortHot || mode == SortPopular {
		fetchLimit = limit * 2
		if fetchLimit < 40 {
			fetchLimit = 40
		}
		if fetchLimit > 50 {
			fetchLimit = 50
		}
	}

	u := "https://export.arxiv.org/api/query?search_query=" + url.QueryEscape("cat:"+category) +
		fmt.Sprintf("&start=0&max_results=%d&sortBy=%s&sortOrder=descending", fetchLimit, arxivSortBy(mode))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("arXiv returned %s", resp.Status)
	}
	var feed atomFeed
	if err = xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	ranked := make([]rankedPaper, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		id := ""
		for _, l := range e.Links {
			if strings.Contains(l.Href, "/abs/") {
				id = canonicalArxivID(strings.TrimSuffix(strings.TrimPrefix(l.Href, "https://arxiv.org/abs/"), "/"))
				break
			}
		}
		if id == "" {
			continue
		}
		published, _ := time.Parse(time.RFC3339, e.Published)
		updated, _ := time.Parse(time.RFC3339, e.Updated)
		if updated.IsZero() {
			updated = published
		}

		var score float64
		switch mode {
		case SortHot:
			score = hotScore(updated)
		case SortPopular:
			score = popularScore(published, updated)
		default:
			score = newScore(published)
		}

		item := TrendingPaper{
			ArxivID: id, Title: strings.Join(strings.Fields(e.Title), " "),
			Abstract:        strPtr(strings.Join(strings.Fields(e.Summary), " ")),
			PublishedAt:     e.Published,
			Category:        category,
			PopularityScore: score,
			PDFURL:          "https://arxiv.org/pdf/" + id + ".pdf",
			AbsURL:          "https://arxiv.org/abs/" + id,
		}
		for _, a := range e.Authors {
			item.Authors = append(item.Authors, strings.Join(strings.Fields(a.Name), " "))
		}
		ranked = append(ranked, rankedPaper{item: item, published: published, updated: updated, score: score})
	}

	switch mode {
	case SortHot:
		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].updated.Equal(ranked[j].updated) {
				return ranked[i].score > ranked[j].score
			}
			return ranked[i].updated.After(ranked[j].updated)
		})
	case SortPopular:
		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].score == ranked[j].score {
				return ranked[i].updated.After(ranked[j].updated)
			}
			return ranked[i].score > ranked[j].score
		})
	default:
		// arXiv уже отдал submittedDate desc — оставляем порядок.
	}

	out := make([]TrendingPaper, 0, limit)
	for _, row := range ranked {
		out = append(out, row.item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
