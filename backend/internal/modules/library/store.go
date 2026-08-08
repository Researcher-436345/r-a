package library

import (
	"context"
	"fmt"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	DB      *pgxpool.Pool
	Catalog catalog.Store
}

func (s Store) Has(ctx context.Context, userID, paperID uuid.UUID) (bool, error) {
	var ok bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_library_items WHERE user_id=$1 AND paper_id=$2)`, userID, paperID).Scan(&ok)
	return ok, err
}

func (s Store) Add(ctx context.Context, userID, paperID uuid.UUID) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO user_library_items (id,user_id,paper_id,status,favorite) VALUES($1,$2,$3,'unread',false) ON CONFLICT(user_id,paper_id) DO NOTHING`, uuid.New(), userID, paperID)
	return err
}

func (s Store) Item(ctx context.Context, userID, paperID uuid.UUID) (ItemOut, error) {
	var x ItemOut
	err := s.DB.QueryRow(ctx, `SELECT id,status,favorite,added_at FROM user_library_items WHERE user_id=$1 AND paper_id=$2`, userID, paperID).Scan(&x.ID, &x.Status, &x.Favorite, &x.AddedAt)
	if err != nil {
		return x, err
	}
	p, err := s.Catalog.GetPaperOut(ctx, paperID)
	x.Paper = p
	return x, err
}

func (s Store) List(ctx context.Context, userID uuid.UUID, page, limit int, status *string) ([]ItemOut, int, error) {
	where := ` WHERE user_id=$1`
	args := []any{userID}
	if status != nil {
		where += ` AND status=$2`
		args = append(args, *status)
	}
	var total int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM user_library_items`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offsetIdx := len(args) + 1
	limitIdx := len(args) + 2
	q := fmt.Sprintf(`SELECT paper_id FROM user_library_items%s ORDER BY added_at DESC OFFSET $%d LIMIT $%d`, where, offsetIdx, limitIdx)
	args = append(args, (page-1)*limit, limit)
	rows, err := s.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]ItemOut, 0)
	for rows.Next() {
		var paperID uuid.UUID
		if err = rows.Scan(&paperID); err != nil {
			return nil, 0, err
		}
		item, err := s.Item(ctx, userID, paperID)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (s Store) Patch(ctx context.Context, userID, paperID uuid.UUID, status *string, favorite *bool) (ItemOut, error) {
	tag, err := s.DB.Exec(ctx, `UPDATE user_library_items SET status=COALESCE($3,status),favorite=COALESCE($4,favorite) WHERE user_id=$1 AND paper_id=$2`, userID, paperID, status, favorite)
	if err != nil {
		return ItemOut{}, err
	}
	if tag.RowsAffected() == 0 {
		return ItemOut{}, pgx.ErrNoRows
	}
	return s.Item(ctx, userID, paperID)
}

func (s Store) Delete(ctx context.Context, userID, paperID uuid.UUID) (bool, error) {
	tag, err := s.DB.Exec(ctx, `DELETE FROM user_library_items WHERE user_id=$1 AND paper_id=$2`, userID, paperID)
	return tag.RowsAffected() > 0, err
}
