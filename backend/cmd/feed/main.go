package main

import (
	"log"
	"net/http"

	"github.com/centraluniversity/researcher/internal/modules/feed"
	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, 200, map[string]string{"status": "ok", "service": "feed"})
	})
	r.Group(func(r chi.Router) {
		r.Use(identity.MiddlewareFromGateway)
		feed.API{Service: feed.Service{
			Redis: redis.NewClient(redisOpts),
			Citations: feed.CitationConfig{
				Enabled:               cfg.CitationsEnabled,
				OpenAlexMailto:        cfg.OpenAlexMailto,
				SemanticScholarAPIKey: cfg.SemanticScholarAPIKey,
			},
		}}.Mount(r)
	})

	log.Printf("feed listening on %s", cfg.HTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, r))
}
