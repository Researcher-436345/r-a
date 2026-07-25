// Package article provides an HTTP client for the upstream Article Service.
package article

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"assistant/internal/domain"
)

// Client fetches article content from the Article Service.
//
// Assumed contract:
//
//	GET {baseURL}/articles/{articleID}
//	200 -> {"id","title","authors","content"}
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates an Article Service client. baseURL is the service root (without a
// trailing slash requirement); httpClient must be non-nil.
func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
	}
}

// Get fetches a single article by its ID.
func (c *Client) Get(ctx context.Context, articleID string) (domain.Article, error) {
	endpoint := fmt.Sprintf("%s/articles/%s", c.baseURL, url.PathEscape(articleID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.Article{}, fmt.Errorf("article: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.Article{}, fmt.Errorf("article: request %s: %w", articleID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return domain.Article{}, fmt.Errorf("article: %s returned %d: %s", articleID, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var a domain.Article
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return domain.Article{}, fmt.Errorf("article: decode %s: %w", articleID, err)
	}
	if a.Content == "" {
		return domain.Article{}, fmt.Errorf("article: %s has empty content", articleID)
	}
	return a, nil
}
