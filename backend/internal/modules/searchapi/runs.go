package searchapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrResearchRunning = errors.New("research is already running in this chat")

type ResearchRun struct {
	ID                 uuid.UUID `json:"id"`
	Status             string    `json:"status"`
	ProviderResponseID *string   `json:"-"`
	ErrorMessage       *string   `json:"error_message,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

func (s Store) StartRun(ctx context.Context, chatID uuid.UUID, content string) (uuid.UUID, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	id := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO search_chat_messages (id, chat_id, role, content) VALUES ($1, $2, 'user', $3)`, id, chatID, content)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO search_runs (id, chat_id) VALUES ($1, $2)`, id, chatID)
	var constraint *pgconn.PgError
	if errors.As(err, &constraint) && constraint.Code == "23505" {
		return uuid.Nil, ErrResearchRunning
	}
	if err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit(ctx)
}

func (s Store) LatestRun(ctx context.Context, userID, chatID uuid.UUID) (*ResearchRun, error) {
	var run ResearchRun
	err := s.DB.QueryRow(ctx, `SELECT r.id, r.status, r.provider_response_id, r.error_message, r.created_at
		FROM search_runs r JOIN search_chats c ON c.id = r.chat_id
		WHERE r.chat_id = $1 AND c.user_id = $2 ORDER BY r.created_at DESC LIMIT 1`, chatID, userID).
		Scan(&run.ID, &run.Status, &run.ProviderResponseID, &run.ErrorMessage, &run.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &run, err
}

func (s Store) SaveResponseID(ctx context.Context, runID uuid.UUID, responseID string) error {
	tag, err := s.DB.Exec(ctx, `UPDATE search_runs SET provider_response_id = $2, updated_at = now() WHERE id = $1 AND status = 'running'`, runID, responseID)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s Store) FailRun(ctx context.Context, runID uuid.UUID, detail string) error {
	_, err := s.DB.Exec(ctx, `UPDATE search_runs SET status = 'failed', error_message = $2, updated_at = now() WHERE id = $1 AND status = 'running'`, runID, detail)
	return err
}

// CompleteRun is shared by streaming and recovery, so a completed response
// can only become one assistant message even when both finish concurrently.
func (s Store) CompleteRun(ctx context.Context, runID uuid.UUID, content string, sources []Source) (Message, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback(ctx)
	var chatID uuid.UUID
	var status string
	var messageID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT chat_id, status, assistant_message_id FROM search_runs WHERE id = $1 FOR UPDATE`, runID).
		Scan(&chatID, &status, &messageID)
	if err != nil {
		return Message{}, err
	}
	if status == "failed" {
		return Message{}, errors.New("research run has failed")
	}
	if sources == nil {
		sources = []Source{}
	}
	raw, err := json.Marshal(sources)
	if err != nil {
		return Message{}, err
	}
	if status == "running" {
		id := uuid.New()
		messageID = &id
		_, err = tx.Exec(ctx, `INSERT INTO search_chat_messages (id, chat_id, role, content, sources) VALUES ($1, $2, 'assistant', $3, $4::jsonb)`, id, chatID, content, string(raw))
		if err != nil {
			return Message{}, err
		}
		_, err = tx.Exec(ctx, `UPDATE search_runs SET status = 'completed', assistant_message_id = $2, updated_at = now() WHERE id = $1`, runID, id)
		if err != nil {
			return Message{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE search_chats SET updated_at = now() WHERE id = $1`, chatID); err != nil {
			return Message{}, err
		}
	}
	var message Message
	err = tx.QueryRow(ctx, `SELECT id, chat_id, role, content, sources, created_at FROM search_chat_messages WHERE id = $1`, messageID).
		Scan(&message.ID, &message.ChatID, &message.Role, &message.Content, &raw, &message.CreatedAt)
	if err != nil {
		return Message{}, err
	}
	if err := json.Unmarshal(raw, &message.Sources); err != nil {
		return Message{}, err
	}
	return message, tx.Commit(ctx)
}
