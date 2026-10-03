package feed

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGDescriptionStore struct{ DB *pgxpool.Pool }

func (s PGDescriptionStore) Brief(ctx context.Context, id, hash string) (string, error) {
	var text string
	err := s.DB.QueryRow(ctx, `SELECT brief_text FROM feed_briefs
		WHERE arxiv_id=$1 AND source_hash=$2`, id, hash).Scan(&text)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return text, err
}

func (s PGDescriptionStore) SaveBrief(ctx context.Context, id, hash, source, text, model string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO feed_briefs
		(arxiv_id, source_hash, source_text, brief_text, model)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, id, hash, source, text, model)
	return err
}

func (s PGDescriptionStore) Snapshot(ctx context.Context, category string, mode SortMode, lang string) ([]TrendingPaper, time.Time, error) {
	var raw []byte
	var updated time.Time
	err := s.DB.QueryRow(ctx, `SELECT items, prepared_at FROM feed_brief_snapshots
		WHERE category=$1 AND sort=$2 AND lang=$3`, category, mode, lang).Scan(&raw, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, updated, nil
	}
	if err != nil {
		return nil, updated, err
	}
	var items []TrendingPaper
	err = json.Unmarshal(raw, &items)
	return items, updated, err
}

func (s PGDescriptionStore) Publish(ctx context.Context, category string, mode SortMode, lang string, items []TrendingPaper) error {
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(ctx, `INSERT INTO feed_brief_snapshots(category, sort, lang, items)
		VALUES ($1,$2,$3,$4) ON CONFLICT (category, sort, lang) DO UPDATE
		SET items=EXCLUDED.items, prepared_at=now()`, category, mode, lang, raw)
	return err
}

func (s PGDescriptionStore) WithRefreshLock(ctx context.Context, refresh func(context.Context) error) error {
	// Транзакционная блокировка совместима с PgBouncer transaction pooling.
	// Сами переводы сохраняются вне этой транзакции: после сбоя продолжаем с готовых.
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var acquired bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(726105, 2)`).Scan(&acquired); err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	return refresh(ctx)
}
