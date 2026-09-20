package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"net/url"
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
		return processRemotePDF(ctx, t, papers, s3, qclient)
	})
	mux.HandleFunc(queue.ProcessRemotePDF, func(ctx context.Context, t *asynq.Task) error {
		return processRemotePDF(ctx, t, papers, s3, qclient)
	})
	mux.HandleFunc(queue.FinalizeUploadedPDF, func(ctx context.Context, t *asynq.Task) error {
		return finalizeUpload(ctx, t, papers, s3, qclient)
	})
	mux.HandleFunc(queue.ProcessPaperParse, func(ctx context.Context, t *asynq.Task) error {
		return processParse(ctx, t, papers, docs, s3, parser)
	})
	log.Fatal(server.Run(mux))
}

func processRemotePDF(ctx context.Context, t *asynq.Task, p catalog.Store, s3 *storage.Client, q *asynq.Client) error {
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
	// Duplicate enqueues (retry-pdf, requeue on dedupe) must not re-download.
	if v.Status == "ready" && v.PDFKey != nil {
		return nil
	}
	if v.SourceURL == nil || strings.TrimSpace(*v.SourceURL) == "" {
		return fail(ctx, p, v, "Missing source URL")
	}
	data, err := catalog.DownloadPDF(ctx, *v.SourceURL)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			// Worker shutdown: asynq requeues the task, so this is not a failure.
			return err
		}
		return fail(ctx, p, v, downloadFailureMessage(*v.SourceURL, err))
	}
	key := storage.PDFKey(v.PaperID.String(), uuid.NewString())
	if err = s3.Upload(ctx, key, data); err != nil {
		return fail(ctx, p, v, "Could not store the downloaded PDF: "+err.Error())
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
		markCtx, cancel := detached(ctx)
		defer cancel()
		return docs.MarkFailed(markCtx, v.PaperID, v.ID, "Missing PDF key")
	}
	_ = docs.UpsertPending(ctx, v.PaperID, v.ID)

	paper, err := p.GetPaperOut(ctx, v.PaperID)
	if err != nil {
		return parseFailed(ctx, docs, v, err)
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
			if err := docs.SaveReady(ctx, v.PaperID, v.ID, tex.Engine, false, tex.PageCount, tex.Markdown, tex.PlainText, storeChunks); err != nil {
				return parseFailed(ctx, docs, v, err)
			}
			return nil
		}
	}

	data, err := s3.Download(ctx, *v.PDFKey)
	if err != nil {
		return parseFailed(ctx, docs, v, err)
	}
	parsed, err := parser.ParsePDF(ctx, data, v.PaperID.String())
	if err != nil {
		return parseFailed(ctx, docs, v, err)
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
	if err := docs.SaveReady(ctx, v.PaperID, v.ID, parsed.Engine, parsed.OCRUsed, parsed.PageCount, md, plain, chunks); err != nil {
		return parseFailed(ctx, docs, v, err)
	}
	return nil
}

// parseFailed records the failure before returning the error, so
// paper_documents never stays 'pending' (the overview waits on it) while
// asynq retries or archives the task.
func parseFailed(ctx context.Context, docs content.Store, v catalog.Version, err error) error {
	markCtx, cancel := detached(ctx)
	defer cancel()
	if markErr := docs.MarkFailed(markCtx, v.PaperID, v.ID, err.Error()); markErr != nil {
		log.Printf("parse %s: mark failed: %v (original error: %v)", v.ID, markErr, err)
	}
	return err
}

func fail(ctx context.Context, p catalog.Store, v catalog.Version, message string) error {
	v.Status = "failed"
	v.ErrorMessage = &message
	updateCtx, cancel := detached(ctx)
	defer cancel()
	return p.UpdateVersion(updateCtx, v)
}

// detached keeps status writes working after the task context timed out or
// was cancelled; otherwise the failure itself could not be recorded.
func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
}

// downloadFailureMessage names the host so the reader/library can show where
// the download was refused (typically 403 from publisher bot protection).
func downloadFailureMessage(sourceURL string, err error) string {
	host := sourceURL
	if u, parseErr := url.Parse(sourceURL); parseErr == nil && u.Host != "" {
		host = u.Host
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "PDF download from " + host + " timed out"
	}
	return "Could not download the PDF from " + host + ": " + err.Error()
}
