package content

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

type Document struct {
	PaperID      uuid.UUID `json:"paper_id"`
	VersionID    uuid.UUID `json:"version_id"`
	Engine       string    `json:"engine"`
	OCRUsed      bool      `json:"ocr_used"`
	PageCount    int       `json:"page_count"`
	Markdown     string    `json:"markdown"`
	PlainText    string    `json:"plain_text"`
	Status       string    `json:"status"`
	ErrorMessage *string   `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Chunk struct {
	ID            uuid.UUID `json:"id"`
	PaperID       uuid.UUID `json:"paper_id"`
	VersionID     uuid.UUID `json:"version_id"`
	ChunkIndex    int       `json:"chunk_index"`
	PageStart     int       `json:"page_start"`
	PageEnd       int       `json:"page_end"`
	Section       *string   `json:"section"`
	Text          string    `json:"text"`
	TokenEstimate int       `json:"token_estimate"`
}

type ThreadSummary struct {
	UserID            uuid.UUID  `json:"user_id"`
	PaperID           uuid.UUID  `json:"paper_id"`
	Summary           string     `json:"summary"`
	CoveredMessageID  *uuid.UUID `json:"covered_message_id"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (s Store) UpsertPending(ctx context.Context, paperID, versionID uuid.UUID) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO paper_documents (paper_id, version_id, engine, status)
		VALUES ($1, $2, 'pending', 'pending')
		ON CONFLICT (paper_id) DO UPDATE SET
			version_id = EXCLUDED.version_id,
			status = 'pending',
			error_message = NULL,
			updated_at = now()`, paperID, versionID)
	return err
}

func (s Store) MarkFailed(ctx context.Context, paperID, versionID uuid.UUID, message string) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO paper_documents (paper_id, version_id, engine, status, error_message)
		VALUES ($1, $2, 'unknown', 'failed', $3)
		ON CONFLICT (paper_id) DO UPDATE SET
			version_id = EXCLUDED.version_id,
			status = 'failed',
			error_message = EXCLUDED.error_message,
			updated_at = now()`, paperID, versionID, message)
	return err
}

func (s Store) SaveReady(
	ctx context.Context,
	paperID, versionID uuid.UUID,
	engine string,
	ocrUsed bool,
	pageCount int,
	markdown, plainText string,
	chunks []Chunk,
) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO paper_documents (
			paper_id, version_id, engine, ocr_used, page_count, markdown, plain_text, status, error_message, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'ready',NULL,now())
		ON CONFLICT (paper_id) DO UPDATE SET
			version_id = EXCLUDED.version_id,
			engine = EXCLUDED.engine,
			ocr_used = EXCLUDED.ocr_used,
			page_count = EXCLUDED.page_count,
			markdown = EXCLUDED.markdown,
			plain_text = EXCLUDED.plain_text,
			status = 'ready',
			error_message = NULL,
			updated_at = now()`,
		paperID, versionID, engine, ocrUsed, pageCount, markdown, plainText)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM paper_chunks WHERE paper_id = $1`, paperID)
	if err != nil {
		return err
	}
	for _, c := range chunks {
		id := c.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO paper_chunks (
				id, paper_id, version_id, chunk_index, page_start, page_end, section, text, token_estimate
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			id, paperID, versionID, c.ChunkIndex, c.PageStart, c.PageEnd, c.Section, c.Text, c.TokenEstimate)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) GetDocument(ctx context.Context, paperID uuid.UUID) (Document, error) {
	var d Document
	err := s.DB.QueryRow(ctx, `
		SELECT paper_id, version_id, engine, ocr_used, page_count, markdown, plain_text, status, error_message, created_at, updated_at
		FROM paper_documents WHERE paper_id = $1`, paperID,
	).Scan(&d.PaperID, &d.VersionID, &d.Engine, &d.OCRUsed, &d.PageCount, &d.Markdown, &d.PlainText, &d.Status, &d.ErrorMessage, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (s Store) GetThreadSummary(ctx context.Context, userID, paperID uuid.UUID) (ThreadSummary, error) {
	var t ThreadSummary
	err := s.DB.QueryRow(ctx, `
		SELECT user_id, paper_id, summary, covered_message_id, updated_at
		FROM chat_thread_summaries WHERE user_id = $1 AND paper_id = $2`, userID, paperID,
	).Scan(&t.UserID, &t.PaperID, &t.Summary, &t.CoveredMessageID, &t.UpdatedAt)
	return t, err
}

func (s Store) UpsertThreadSummary(ctx context.Context, userID, paperID uuid.UUID, summary string, coveredMessageID *uuid.UUID) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO chat_thread_summaries (user_id, paper_id, summary, covered_message_id, updated_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (user_id, paper_id) DO UPDATE SET
			summary = EXCLUDED.summary,
			covered_message_id = EXCLUDED.covered_message_id,
			updated_at = now()`, userID, paperID, summary, coveredMessageID)
	return err
}
