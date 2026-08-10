package assistant

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

type Message struct {
	ID          uuid.UUID `json:"id"`
	PaperID     uuid.UUID `json:"paper_id"`
	UserID      uuid.UUID `json:"user_id"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	ContextText *string   `json:"context_text"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s Store) List(ctx context.Context, userID, paperID uuid.UUID) ([]Message, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, paper_id, user_id, role, content, context_text, created_at
		FROM chat_messages
		WHERE user_id = $1 AND paper_id = $2
		ORDER BY created_at ASC, id ASC`, userID, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.PaperID, &m.UserID, &m.Role, &m.Content, &m.ContextText, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListRecent returns the last limit messages in chronological order.
func (s Store) ListRecent(ctx context.Context, userID, paperID uuid.UUID, limit int) ([]Message, error) {
	if limit < 1 {
		limit = 20
	}
	rows, err := s.DB.Query(ctx, `
		SELECT id, paper_id, user_id, role, content, context_text, created_at
		FROM (
			SELECT id, paper_id, user_id, role, content, context_text, created_at
			FROM chat_messages
			WHERE user_id = $1 AND paper_id = $2
			ORDER BY created_at DESC, id DESC
			LIMIT $3
		) recent
		ORDER BY created_at ASC, id ASC`, userID, paperID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Message, 0, limit)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.PaperID, &m.UserID, &m.Role, &m.Content, &m.ContextText, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s Store) AppendPair(
	ctx context.Context,
	userID, paperID uuid.UUID,
	userContent string,
	contextText *string,
	assistantContent string,
) (userMsg Message, assistantMsg Message, err error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return userMsg, assistantMsg, err
	}
	defer tx.Rollback(ctx)

	userMsgID := uuid.New()
	assistantMsgID := uuid.New()

	err = tx.QueryRow(ctx, `
		INSERT INTO chat_messages (id, paper_id, user_id, role, content, context_text)
		VALUES ($1, $2, $3, 'user', $4, $5)
		RETURNING id, paper_id, user_id, role, content, context_text, created_at`,
		userMsgID, paperID, userID, userContent, contextText,
	).Scan(&userMsg.ID, &userMsg.PaperID, &userMsg.UserID, &userMsg.Role, &userMsg.Content, &userMsg.ContextText, &userMsg.CreatedAt)
	if err != nil {
		return userMsg, assistantMsg, err
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO chat_messages (id, paper_id, user_id, role, content, context_text)
		VALUES ($1, $2, $3, 'assistant', $4, NULL)
		RETURNING id, paper_id, user_id, role, content, context_text, created_at`,
		assistantMsgID, paperID, userID, assistantContent,
	).Scan(&assistantMsg.ID, &assistantMsg.PaperID, &assistantMsg.UserID, &assistantMsg.Role, &assistantMsg.Content, &assistantMsg.ContextText, &assistantMsg.CreatedAt)
	if err != nil {
		return userMsg, assistantMsg, err
	}

	if err := tx.Commit(ctx); err != nil {
		return userMsg, assistantMsg, err
	}
	return userMsg, assistantMsg, nil
}
