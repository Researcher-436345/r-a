package searchapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

type Source struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Domain      string  `json:"domain"`
	PublishedAt *string `json:"published_at"`
}

type Message struct {
	ID        uuid.UUID `json:"id"`
	ChatID    uuid.UUID `json:"chat_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Sources   []Source  `json:"sources"`
	CreatedAt time.Time `json:"created_at"`
}

type Chat struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Mode      string    `json:"mode"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages,omitempty"`
}

func (s Store) List(ctx context.Context, userID uuid.UUID) ([]Chat, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, title, mode, created_at, updated_at
		FROM search_chats
		WHERE user_id = $1
		ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Chat, 0)
	for rows.Next() {
		var chat Chat
		if err := rows.Scan(&chat.ID, &chat.Title, &chat.Mode, &chat.CreatedAt, &chat.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, chat)
	}
	return out, rows.Err()
}

func (s Store) Get(ctx context.Context, userID, chatID uuid.UUID) (Chat, error) {
	var chat Chat
	err := s.DB.QueryRow(ctx, `
		SELECT id, title, mode, created_at, updated_at
		FROM search_chats WHERE id = $1 AND user_id = $2`, chatID, userID,
	).Scan(&chat.ID, &chat.Title, &chat.Mode, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return chat, err
	}
	chat.Messages, err = s.Messages(ctx, chatID, 200)
	return chat, err
}

func (s Store) Delete(ctx context.Context, userID, chatID uuid.UUID) (bool, error) {
	tag, err := s.DB.Exec(ctx, `DELETE FROM search_chats WHERE id = $1 AND user_id = $2`, chatID, userID)
	return tag.RowsAffected() > 0, err
}

func (s Store) Ensure(ctx context.Context, userID, chatID uuid.UUID, title, mode string) error {
	tag, err := s.DB.Exec(ctx, `
		INSERT INTO search_chats (id, user_id, title, mode)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`, chatID, userID, title, mode)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var owner uuid.UUID
	if err := s.DB.QueryRow(ctx, `SELECT user_id FROM search_chats WHERE id = $1`, chatID).Scan(&owner); err != nil {
		return err
	}
	if owner != userID {
		return pgx.ErrNoRows
	}
	_, err = s.DB.Exec(ctx, `UPDATE search_chats SET mode = $2, updated_at = now() WHERE id = $1`, chatID, mode)
	return err
}

func (s Store) AddMessage(ctx context.Context, chatID uuid.UUID, role, content string, sources []Source) (Message, error) {
	if sources == nil {
		sources = []Source{}
	}
	raw, err := json.Marshal(sources)
	if err != nil {
		return Message{}, err
	}
	var message Message
	var storedSources []byte
	err = s.DB.QueryRow(ctx, `
		INSERT INTO search_chat_messages (id, chat_id, role, content, sources)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		RETURNING id, chat_id, role, content, sources, created_at`,
		uuid.New(), chatID, role, content, string(raw),
	).Scan(&message.ID, &message.ChatID, &message.Role, &message.Content, &storedSources, &message.CreatedAt)
	if err != nil {
		return message, err
	}
	if err := json.Unmarshal(storedSources, &message.Sources); err != nil {
		return message, err
	}
	_, err = s.DB.Exec(ctx, `UPDATE search_chats SET updated_at = now() WHERE id = $1`, chatID)
	return message, err
}

func (s Store) Messages(ctx context.Context, chatID uuid.UUID, limit int) ([]Message, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, chat_id, role, content, sources, created_at FROM (
			SELECT id, chat_id, role, content, sources, created_at
			FROM search_chat_messages WHERE chat_id = $1
			ORDER BY created_at DESC, id DESC LIMIT $2
		) recent ORDER BY created_at, id`, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Message, 0)
	for rows.Next() {
		var message Message
		var raw []byte
		if err := rows.Scan(&message.ID, &message.ChatID, &message.Role, &message.Content, &raw, &message.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &message.Sources); err != nil {
			return nil, err
		}
		out = append(out, message)
	}
	return out, rows.Err()
}
