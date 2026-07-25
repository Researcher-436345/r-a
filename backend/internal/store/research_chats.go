package store

import (
	"context"
	"errors"

	"github.com/centraluniversity/researcher/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ResearchChats struct{ DB *pgxpool.Pool }

func (s ResearchChats) List(ctx context.Context, userID uuid.UUID) ([]models.ResearchChat, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, title, mode, created_at, updated_at
		FROM research_chats
		WHERE user_id = $1
		ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make([]models.ResearchChat, 0)
	for rows.Next() {
		var chat models.ResearchChat
		if err := rows.Scan(&chat.ID, &chat.Title, &chat.Mode, &chat.CreatedAt, &chat.UpdatedAt); err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

func (s ResearchChats) Get(ctx context.Context, userID, chatID uuid.UUID) (models.ResearchChat, error) {
	var chat models.ResearchChat
	err := s.DB.QueryRow(ctx, `
		SELECT id, title, mode, created_at, updated_at
		FROM research_chats
		WHERE id = $1 AND user_id = $2`, chatID, userID,
	).Scan(&chat.ID, &chat.Title, &chat.Mode, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return chat, err
	}

	chat.Messages, err = s.Messages(ctx, chatID, 200)
	return chat, err
}

func (s ResearchChats) Ensure(ctx context.Context, userID, chatID uuid.UUID, title, mode string) error {
	tag, err := s.DB.Exec(ctx, `
		INSERT INTO research_chats (id, user_id, title, mode)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`, chatID, userID, title, mode)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	var ownerID uuid.UUID
	err = s.DB.QueryRow(ctx, `SELECT user_id FROM research_chats WHERE id = $1`, chatID).Scan(&ownerID)
	if err != nil {
		return err
	}
	if ownerID != userID {
		return pgx.ErrNoRows
	}
	_, err = s.DB.Exec(ctx, `
		UPDATE research_chats SET mode = $2, updated_at = now()
		WHERE id = $1`, chatID, mode)
	return err
}

func (s ResearchChats) AddMessage(ctx context.Context, chatID uuid.UUID, role, content string) (models.ResearchChatMessage, error) {
	var message models.ResearchChatMessage
	err := s.DB.QueryRow(ctx, `
		INSERT INTO research_chat_messages (id, chat_id, role, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, chat_id, role, content, created_at`,
		uuid.New(), chatID, role, content,
	).Scan(&message.ID, &message.ChatID, &message.Role, &message.Content, &message.CreatedAt)
	if err == nil {
		_, err = s.DB.Exec(ctx, `UPDATE research_chats SET updated_at = now() WHERE id = $1`, chatID)
	}
	return message, err
}

func (s ResearchChats) Messages(ctx context.Context, chatID uuid.UUID, limit int) ([]models.ResearchChatMessage, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, chat_id, role, content, created_at
		FROM (
			SELECT id, chat_id, role, content, created_at
			FROM research_chat_messages
			WHERE chat_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		) recent
		ORDER BY created_at, id`, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]models.ResearchChatMessage, 0)
	for rows.Next() {
		var message models.ResearchChatMessage
		if err := rows.Scan(&message.ID, &message.ChatID, &message.Role, &message.Content, &message.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
