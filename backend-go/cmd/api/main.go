package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/centraluniversity/researcher/internal/config"
	"github.com/centraluniversity/researcher/internal/db"
	"github.com/centraluniversity/researcher/internal/httpapi"
	"github.com/centraluniversity/researcher/internal/queue"
	"github.com/centraluniversity/researcher/internal/storage"
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
	s3, err := storage.New(cfg.S3Endpoint, cfg.S3PublicEndpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3Region, cfg.S3PresignExpire)
	if err != nil {
		log.Fatal(err)
	}
	if err := s3.EnsureBucket(ctx); err != nil {
		log.Printf("ensure bucket: %v", err)
	}
	q, err := queue.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	defer q.Close()
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	server := httpapi.Server{Config: cfg, DB: pool, Storage: s3, Queue: q, Redis: redis.NewClient(redisOpts)}
	log.Printf("%s listening on %s", cfg.AppName, cfg.HTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, server.Router()))
}
