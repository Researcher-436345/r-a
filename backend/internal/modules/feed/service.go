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
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Service struct{ Redis *redis.Client }

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
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

func (f Service) Trending(ctx context.Context, category string, limit int) ([]TrendingPaper, bool, error) {
	if category == "" {
		category = "cs.AI"
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	key := fmt.Sprintf("feed:trending:%s:%d", category, limit)
	if f.Redis != nil {
		if cached, err := f.Redis.Get(ctx, key).Result(); err == nil {
			var items []TrendingPaper
			if json.Unmarshal([]byte(cached), &items) == nil {
				return items, true, nil
			}
		}
	}
	items, err := fetchTrending(ctx, category, limit)
	if err != nil {
		return nil, false, err
	}
	if f.Redis != nil {
		if b, e := json.Marshal(items); e == nil {
			_ = f.Redis.Set(ctx, key, b, time.Hour).Err()
		}
	}
	return items, false, nil
}

func popularity(t time.Time) float64 {
	return math.Max(1, math.Floor(1000*math.Exp(-time.Since(t).Hours()/24/14)))
}

func canonicalArxivID(id string) string {
	return regexp.MustCompile(`(?i)v\d+$`).ReplaceAllString(id, "")
}

func fetchTrending(ctx context.Context, category string, limit int) ([]TrendingPaper, error) {
	u := "https://export.arxiv.org/api/query?search_query=" + url.QueryEscape("cat:"+category) + fmt.Sprintf("&start=0&max_results=%d&sortBy=submittedDate&sortOrder=descending", limit)
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
	out := make([]TrendingPaper, 0, len(feed.Entries))
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
		item := TrendingPaper{
			ArxivID: id, Title: strings.Join(strings.Fields(e.Title), " "),
			Abstract: strPtr(strings.Join(strings.Fields(e.Summary), " ")),
			PublishedAt: e.Published, Category: category, PopularityScore: popularity(published),
			PDFURL: "https://arxiv.org/pdf/" + id + ".pdf", AbsURL: "https://arxiv.org/abs/" + id,
		}
		for _, a := range e.Authors {
			item.Authors = append(item.Authors, strings.Join(strings.Fields(a.Name), " "))
		}
		out = append(out, item)
	}
	return out, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
