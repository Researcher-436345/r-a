package annotations

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

const annotationCols = `id,paper_id,page,rect,selected_text,note,color,source_chat_message_id,created_at,updated_at`

func (s Store) List(ctx context.Context, userID, paperID uuid.UUID) ([]Out, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+annotationCols+` FROM annotations WHERE user_id=$1 AND paper_id=$2 ORDER BY page,created_at`, userID, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Out, 0)
	for rows.Next() {
		a, err := scanAnnotation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s Store) Create(
	ctx context.Context,
	userID, paperID uuid.UUID,
	page int,
	rect *Rect,
	text, note, color string,
	sourceChatMessageID *uuid.UUID,
) (Out, error) {
	var raw []byte
	if rect != nil {
		raw, _ = json.Marshal(rect)
	}
	return s.scan(ctx, `INSERT INTO annotations(id,paper_id,user_id,page,rect,selected_text,note,color,source_chat_message_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+annotationCols,
		uuid.New(), paperID, userID, page, raw, text, note, color, sourceChatMessageID)
}

func (s Store) Patch(ctx context.Context, userID, id uuid.UUID, note string) (Out, error) {
	return s.scan(ctx, `UPDATE annotations SET note=$3,updated_at=now() WHERE id=$1 AND user_id=$2 RETURNING `+annotationCols, id, userID, note)
}

func (s Store) Delete(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	tag, err := s.DB.Exec(ctx, `DELETE FROM annotations WHERE id=$1 AND user_id=$2`, id, userID)
	return tag.RowsAffected() > 0, err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanAnnotation(row scannable) (Out, error) {
	var a Out
	var raw []byte
	err := row.Scan(&a.ID, &a.PaperID, &a.Page, &raw, &a.SelectedText, &a.Note, &a.Color, &a.SourceChatMessageID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, err
	}
	a.Rect = RectFromJSON(raw)
	return a, nil
}

func (s Store) scan(ctx context.Context, q string, args ...any) (Out, error) {
	return scanAnnotation(s.DB.QueryRow(ctx, q, args...))
}
