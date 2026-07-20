package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"

	"github.com/centraluniversity/researcher/internal/config"
	"github.com/centraluniversity/researcher/internal/db"
	"github.com/centraluniversity/researcher/internal/queue"
	"github.com/centraluniversity/researcher/internal/services"
	"github.com/centraluniversity/researcher/internal/storage"
	"github.com/centraluniversity/researcher/internal/store"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
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
	server, err := queue.NewServer(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	mux := asynq.NewServeMux()
	papers := store.Papers{DB: pool}
	mux.HandleFunc(queue.ProcessArxivPDF, func(ctx context.Context, t *asynq.Task) error { return processArxiv(ctx, t, papers, s3) })
	mux.HandleFunc(queue.FinalizeUploadedPDF, func(ctx context.Context, t *asynq.Task) error { return finalizeUpload(ctx, t, papers, s3) })
	log.Fatal(server.Run(mux))
}
func processArxiv(ctx context.Context, t *asynq.Task, p store.Papers, s3 *storage.Client) error {
	payload, err := queue.Decode(t)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(payload.VersionID)
	if err != nil {
		return err
	}
	v, err := p.GetVersion(ctx, id)
	if err != nil {
		return err
	}
	if v.SourceURL == nil {
		return fail(ctx, p, v, "Missing source URL")
	}
	data, err := services.DownloadPDF(ctx, *v.SourceURL)
	if err != nil {
		return fail(ctx, p, v, err.Error())
	}
	key := storage.PDFKey(v.PaperID.String(), uuid.NewString())
	if err = s3.Upload(ctx, key, data); err != nil {
		return fail(ctx, p, v, err.Error())
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	size := int64(len(data))
	v.PDFKey = &key
	v.SHA256 = &sha
	v.SizeBytes = &size
	v.Status = "ready"
	v.ErrorMessage = nil
	return p.UpdateVersion(ctx, v)
}
func finalizeUpload(ctx context.Context, t *asynq.Task, p store.Papers, s3 *storage.Client) error {
	payload, err := queue.Decode(t)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(payload.VersionID)
	if err != nil {
		return err
	}
	v, err := p.GetVersion(ctx, id)
	if err != nil {
		return err
	}
	if v.PDFKey == nil {
		return fail(ctx, p, v, "Missing PDF key")
	}
	data, err := s3.Download(ctx, *v.PDFKey)
	if err != nil {
		return fail(ctx, p, v, err.Error())
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	size := int64(len(data))
	v.SHA256 = &sha
	v.SizeBytes = &size
	v.Status = "ready"
	v.ErrorMessage = nil
	return p.UpdateVersion(ctx, v)
}
func fail(ctx context.Context, p store.Papers, v store.Version, message string) error {
	v.Status = "failed"
	v.ErrorMessage = &message
	return p.UpdateVersion(ctx, v)
}
