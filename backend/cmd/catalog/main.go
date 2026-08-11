package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/modules/library"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/db"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/queue"
	"github.com/centraluniversity/researcher/internal/platform/storage"
	"github.com/go-chi/chi/v5"
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
	s3, err := storage.New(cfg.S3Endpoint, cfg.S3PublicEndpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3Region, cfg.S3PresignExpire)
	if err != nil {
		log.Fatal(err)
	}
	q, err := queue.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	defer q.Close()

	libStore := library.Store{DB: pool, Catalog: catalog.Store{DB: pool}}
	api := catalog.API{
		DB:         pool,
		Storage:    s3,
		Queue:      q,
		Membership: libStore,
	}

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, 200, map[string]string{"status": "ok", "service": "catalog"})
	})
	api.MountInternal(r)
	r.Group(func(r chi.Router) {
		r.Use(identity.MiddlewareFromGateway)
		api.Mount(r)
	})

	log.Printf("catalog listening on %s", cfg.HTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, r))
}
