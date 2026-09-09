package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/db"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/mailer"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	api := identity.API{
		Config: cfg,
		DB:     pool,
		Redis:  rdb,
		Mail:   mailer.FromConfig(cfg),
	}
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, 200, map[string]string{"status": "ok", "service": "identity"})
	})
	api.Mount(r)

	log.Printf("identity listening on %s", cfg.HTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, r))
}
