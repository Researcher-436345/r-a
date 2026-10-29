package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/analytics"
	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/modules/library"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/db"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/observability"
	"github.com/centraluniversity/researcher/internal/platform/throttle"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func main() {
	observability.Start("library")
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	analytics.Bind(pool)
	analytics.StartAggregator(pool)
	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(redisOptions)
	defer rdb.Close()

	store := library.Store{DB: pool, Catalog: catalog.Store{DB: pool}}
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, 200, map[string]string{"status": "ok", "service": "library"})
	})
	r.Group(func(r chi.Router) {
		r.Use(identity.MiddlewareFromGateway)
		library.API{Store: store}.Mount(r)
		analytics.API{DB: pool, Limiter: &throttle.Limiter{Redis: rdb}}.Mount(r)
	})

	log.Printf("library listening on %s", cfg.HTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, observability.HTTP("library", r)))
}
