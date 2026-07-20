package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}

func Check(ctx context.Context, pool *pgxpool.Pool) bool {
	if pool == nil {
		return false
	}
	return pool.Ping(ctx) == nil
}

func MigrationsOK(ctx context.Context, pool *pgxpool.Pool) bool {
	var v string
	err := pool.QueryRow(ctx, `SELECT version_num FROM alembic_version LIMIT 1`).Scan(&v)
	return err == nil && v != ""
}
