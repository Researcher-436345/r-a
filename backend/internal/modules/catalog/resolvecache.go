package catalog

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// resolveMisses remembers links that did not resolve (not_found, not_a_paper)
// for a few minutes: chat prefetch and page reloads re-send the same links,
// and every miss costs a page fetch plus several index calls.
var resolveMisses = newMissCache(5*time.Minute, 4096)

type missCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	entries map[string]missEntry
	now     func() time.Time
}

type missEntry struct {
	err error
	at  time.Time
}

func newMissCache(ttl time.Duration, max int) *missCache {
	return &missCache{ttl: ttl, max: max, entries: map[string]missEntry{}, now: time.Now}
}

// resolveMissKey is empty for URLs that do not normalize; those fail fast anyway.
func resolveMissKey(rawURL, hint string) string {
	normalized, err := normalizeArticleURL(rawURL)
	if err != nil {
		return ""
	}
	return normalized + "\n" + strings.ToLower(hint)
}

func (c *missCache) get(key string) error {
	if key == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	if c.now().Sub(entry.at) > c.ttl {
		delete(c.entries, key)
		return nil
	}
	return entry.err
}

// remember keeps only answers another attempt would repeat; upstream failures
// (502) stay retryable.
func (c *missCache) remember(key string, err error) {
	if key == "" || !(errors.Is(err, errUnsupportedArticleURL) || errors.Is(err, errNotAPaper)) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.entries) >= c.max {
		for k, e := range c.entries {
			if now.Sub(e.at) > c.ttl {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= c.max {
			c.entries = map[string]missEntry{}
		}
	}
	c.entries[key] = missEntry{err: err, at: now}
}
