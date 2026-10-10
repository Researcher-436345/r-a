package feed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var ErrFeedPreparing = errors.New("feed is preparing")

const feedLanguage = "ru"
const preparedFeedLimit = 50

func NewService(cfg config.Config, pool *pgxpool.Pool, redisClient *redis.Client) Service {
	return Service{
		ParserURL: cfg.ParserServiceURL, Redis: redisClient,
		Citations:    CitationConfig{Enabled: cfg.CitationsEnabled, OpenAlexMailto: cfg.OpenAlexMailto, SemanticScholarAPIKey: cfg.SemanticScholarAPIKey},
		Descriptions: NewDescriptions(cfg, PGDescriptionStore{DB: pool}),
	}
}

type DescriptionStore interface {
	Brief(context.Context, string, string) (string, error)
	SaveBrief(context.Context, string, string, string, string, string) error
	Snapshot(context.Context, string, SortMode, string) ([]TrendingPaper, time.Time, error)
	Publish(context.Context, string, SortMode, string, []TrendingPaper) error
	WithRefreshLock(context.Context, func(context.Context) error) error
}

type Descriptions struct {
	Store           DescriptionStore
	Generator       briefGenerator
	Model           string
	RefreshInterval time.Duration
	Enabled         bool
}

func NewDescriptions(cfg config.Config, store DescriptionStore) *Descriptions {
	return &Descriptions{
		Store: store, Generator: gemmaBriefGenerator{Config: cfg},
		Model: cfg.FeedTranslationModel, RefreshInterval: cfg.FeedRefreshInterval,
		Enabled: strings.TrimSpace(cfg.FeedTranslationAPIKey) != "",
	}
}

// LocalizedTrending читает только готовый снимок; запрос пользователя не вызывает LLM.
func (f Service) LocalizedTrending(ctx context.Context, category string, limit int, mode SortMode, lang string) ([]TrendingPaper, bool, error) {
	if lang == "en" {
		return f.Trending(ctx, category, limit, mode)
	}
	if f.Descriptions == nil {
		return nil, false, ErrFeedPreparing
	}
	items, _, err := f.Descriptions.Store.Snapshot(ctx, category, mode, lang)
	if err != nil {
		return nil, false, err
	}
	if len(items) == 0 {
		return nil, false, ErrFeedPreparing
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, true, nil
}

func abstractHash(text string) string {
	sum := sha256.Sum256([]byte(strings.Join(strings.Fields(text), " ")))
	return hex.EncodeToString(sum[:])
}

func (d *Descriptions) prepare(ctx context.Context, items []TrendingPaper) ([]TrendingPaper, error) {
	prepared := append([]TrendingPaper(nil), items...)
	for i := range prepared {
		p := &prepared[i]
		if p.Abstract == nil || strings.TrimSpace(*p.Abstract) == "" {
			continue
		}
		source := strings.TrimSpace(*p.Abstract)
		hash := abstractHash("brief-v1\n" + p.Title + "\n" + source)
		text, err := d.Store.Brief(ctx, p.ArxivID, hash)
		if err != nil {
			return nil, err
		}
		if text == "" {
			result, err := d.Generator.Generate(ctx, p.Title, source)
			if err != nil {
				return nil, fmt.Errorf("summarize %s: %w", p.ArxivID, err)
			}
			text = strings.TrimSpace(result)
			if text == "" {
				return nil, fmt.Errorf("empty brief")
			}
			if err := d.Store.SaveBrief(ctx, p.ArxivID, hash, source, text, d.Model); err != nil {
				return nil, err
			}
		}
		p.Abstract = strPtr(text)
	}
	return prepared, nil
}

// WarmDescriptions запускается при старте feed/API. Один процесс готовит ленту,
// остальные используют те же переводы и снимки из PostgreSQL.
func (f Service) WarmDescriptions(ctx context.Context) {
	d := f.Descriptions
	if d == nil {
		return
	}
	if !d.Enabled {
		log.Print("feed briefs disabled: configure FEED_TRANSLATION_API_KEY or LLM_API_KEY; existing snapshots remain readable")
		return
	}
	delay := time.Minute
	for {
		workCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		err := f.refreshDescriptions(workCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("prepare feed descriptions: %v", err)
			delay = min(delay*2, 15*time.Minute)
		} else {
			delay = time.Minute
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (f Service) refreshDescriptions(ctx context.Context) error {
	d := f.Descriptions
	return d.Store.WithRefreshLock(ctx, func(ctx context.Context) error {
		for _, mode := range []SortMode{SortNew, SortHot, SortPopular} {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, updated, err := d.Store.Snapshot(ctx, "cs.AI", mode, feedLanguage)
			if err != nil {
				return err
			}
			if !updated.IsZero() && time.Since(updated) < d.RefreshInterval {
				continue
			}
			fetchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			items, _, err := f.Trending(fetchCtx, "cs.AI", preparedFeedLimit, mode)
			cancel()
			if err == nil && len(items) > 0 {
				var prepared []TrendingPaper
				prepared, err = d.prepare(ctx, items)
				if err == nil {
					err = d.Store.Publish(ctx, "cs.AI", mode, feedLanguage, prepared)
				}
			}
			if err != nil {
				// При сбое провайдера не повторяем те же платные запросы для другой сортировки.
				return fmt.Errorf("%s: %w", mode, err)
			}
		}
		return nil
	})
}
