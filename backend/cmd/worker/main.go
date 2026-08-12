package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/modules/content"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/db"
	"github.com/centraluniversity/researcher/internal/platform/queue"
	"github.com/centraluniversity/researcher/internal/platform/storage"
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
	if err := s3.EnsureBucket(ctx); err != nil {
		log.Fatalf("ensure S3 bucket %q: %v", cfg.S3Bucket, err)
	}
	qclient, err := queue.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	defer qclient.Close()
	server, err := queue.NewServer(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	mux := asynq.NewServeMux()
	papers := catalog.Store{DB: pool}
	docs := content.Store{DB: pool}
	parser := content.Client{
		BaseURL: cfg.ParserServiceURL,
		OCR:     cfg.ParserOCR,
		HTTP:    &http.Client{Timeout: cfg.ParserTimeout},
	}
	mux.HandleFunc(queue.ProcessArxivPDF, func(ctx context.Context, t *asynq.Task) error {
		return processArxiv(ctx, t, papers, s3, qclient)
	})
	mux.HandleFunc(queue.FinalizeUploadedPDF, func(ctx context.Context, t *asynq.Task) error {
		return finalizeUpload(ctx, t, papers, s3, qclient)
	})
	mux.HandleFunc(queue.ProcessPaperParse, func(ctx context.Context, t *asynq.Task) error {
		return processParse(ctx, t, papers, docs, s3, parser)
	})
	log.Fatal(server.Run(mux))
}

func processArxiv(ctx context.Context, t *asynq.Task, p catalog.Store, s3 *storage.Client, q *asynq.Client) error {
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
	data, err := catalog.DownloadPDF(ctx, *v.SourceURL)
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
	if err = p.UpdateVersion(ctx, v); err != nil {
		return err
	}
	return enqueueParse(q, v.ID)
}

func finalizeUpload(ctx context.Context, t *asynq.Task, p catalog.Store, s3 *storage.Client, q *asynq.Client) error {
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
	if err = p.UpdateVersion(ctx, v); err != nil {
		return err
	}
	return enqueueParse(q, v.ID)
}

func enqueueParse(q *asynq.Client, versionID uuid.UUID) error {
	if q == nil {
		return nil
	}
	return queue.Enqueue(q, queue.ProcessPaperParse, versionID.String())
}

func processParse(
	ctx context.Context,
	t *asynq.Task,
	p catalog.Store,
	docs content.Store,
	s3 *storage.Client,
	parser content.Client,
) error {
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
		return docs.MarkFailed(ctx, v.PaperID, v.ID, "Missing PDF key")
	}
	_ = docs.UpsertPending(ctx, v.PaperID, v.ID)

	paper, err := p.GetPaperOut(ctx, v.PaperID)
	if err != nil {
		_ = docs.MarkFailed(ctx, v.PaperID, v.ID, err.Error())
		return err
	}

	// Prefer arXiv TeX source when available; fall back to PDF parser.
	if paper.ArxivID != nil && strings.TrimSpace(*paper.ArxivID) != "" {
		if tex, ok, texErr := content.TryArxivTeX(ctx, *paper.ArxivID); texErr == nil && ok {
			chunks := content.ChunkPlainText(tex.PlainText, 1000)
			storeChunks := make([]content.Chunk, 0, len(chunks))
			for i, c := range chunks {
				section := strings.TrimSpace(c.Section)
				var sectionPtr *string
				if section != "" {
					sectionPtr = &section
				}
				storeChunks = append(storeChunks, content.Chunk{
					ID:            uuid.New(),
					PaperID:       v.PaperID,
					VersionID:     v.ID,
					ChunkIndex:    i,
					PageStart:     c.PageStart,
					PageEnd:       c.PageEnd,
					Section:       sectionPtr,
					Text:          c.Text,
					TokenEstimate: c.TokenEstimate,
				})
			}
			return docs.SaveReady(ctx, v.PaperID, v.ID, tex.Engine, false, tex.PageCount, tex.Markdown, tex.PlainText, storeChunks)
		}
	}

	data, err := s3.Download(ctx, *v.PDFKey)
	if err != nil {
		_ = docs.MarkFailed(ctx, v.PaperID, v.ID, err.Error())
		return err
	}
	parsed, err := parser.ParsePDF(ctx, data, v.PaperID.String())
	if err != nil {
		_ = docs.MarkFailed(ctx, v.PaperID, v.ID, err.Error())
		return err
	}
	chunks := make([]content.Chunk, 0, len(parsed.Chunks))
	for i, c := range parsed.Chunks {
		section := strings.TrimSpace(c.Section)
		var sectionPtr *string
		if section != "" {
			sectionPtr = &section
		}
		chunks = append(chunks, content.Chunk{
			ID:            uuid.New(),
			PaperID:       v.PaperID,
			VersionID:     v.ID,
			ChunkIndex:    i,
			PageStart:     c.PageStart,
			PageEnd:       c.PageEnd,
			Section:       sectionPtr,
			Text:          c.Text,
			TokenEstimate: c.TokenEstimate,
		})
	}
	plain := strings.TrimSpace(parsed.PlainText)
	md := strings.TrimSpace(parsed.Markdown)
	if plain == "" {
		plain = md
	}
	return docs.SaveReady(ctx, v.PaperID, v.ID, parsed.Engine, parsed.OCRUsed, parsed.PageCount, md, plain, chunks)
}

func fail(ctx context.Context, p catalog.Store, v catalog.Version, message string) error {
	v.Status = "failed"
	v.ErrorMessage = &message
	return p.UpdateVersion(ctx, v)
}
