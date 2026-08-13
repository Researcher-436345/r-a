package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrSystemFolderDeletion = errors.New("system folders cannot be deleted")

var systemFolders = []struct {
	Key  string
	Name string
}{
	{Key: "want_to_read", Name: "Хочу прочитать"},
	{Key: "reading", Name: "Читаю сейчас"},
	{Key: "other", Name: "Другое"},
}

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
	folders, err := s.EnsureSystemFolders(ctx, userID)
	if err != nil {
		return err
	}
	wantToRead := folders["want_to_read"]
	_, err = s.DB.Exec(ctx, `INSERT INTO user_library_items (id,user_id,paper_id,status,favorite,folder_id) VALUES($1,$2,$3,'unread',false,$4) ON CONFLICT(user_id,paper_id) DO NOTHING`, uuid.New(), userID, paperID, wantToRead)
	return err
}

func (s Store) Item(ctx context.Context, userID, paperID uuid.UUID) (ItemOut, error) {
	var x ItemOut
	err := s.DB.QueryRow(ctx, `SELECT id,status,favorite,folder_id,added_at FROM user_library_items WHERE user_id=$1 AND paper_id=$2`, userID, paperID).Scan(&x.ID, &x.Status, &x.Favorite, &x.FolderID, &x.AddedAt)
	if err != nil {
		return x, err
	}
	p, err := s.Catalog.GetPaperOut(ctx, paperID)
	x.Paper = p
	return x, err
}

func (s Store) List(ctx context.Context, userID uuid.UUID, page, limit int, status *string, folderID *uuid.UUID) ([]ItemOut, int, error) {
	if _, err := s.EnsureSystemFolders(ctx, userID); err != nil {
		return nil, 0, err
	}
	where := ` WHERE user_id=$1`
	args := []any{userID}
	if status != nil {
		where += fmt.Sprintf(` AND status=$%d`, len(args)+1)
		args = append(args, *status)
	}
	if folderID != nil {
		where += fmt.Sprintf(` AND folder_id=$%d`, len(args)+1)
		args = append(args, *folderID)
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

func (s Store) Patch(ctx context.Context, userID, paperID uuid.UUID, status *string, favorite *bool, folderID *uuid.UUID) (ItemOut, error) {
	if folderID != nil {
		var systemKey *string
		if err := s.DB.QueryRow(ctx, `SELECT system_key FROM library_folders WHERE id=$1 AND user_id=$2`, *folderID, userID).Scan(&systemKey); err != nil {
			return ItemOut{}, err
		}
		if systemKey != nil {
			mapped := map[string]string{"want_to_read": "unread", "reading": "reading", "other": "read"}[*systemKey]
			status = &mapped
		}
	}
	tag, err := s.DB.Exec(ctx, `UPDATE user_library_items SET status=COALESCE($3,status),favorite=COALESCE($4,favorite),folder_id=COALESCE($5::uuid,folder_id) WHERE user_id=$1 AND paper_id=$2`, userID, paperID, status, favorite, folderID)
	if err != nil {
		return ItemOut{}, err
	}
	if tag.RowsAffected() == 0 {
		return ItemOut{}, pgx.ErrNoRows
	}
	return s.Item(ctx, userID, paperID)
}

func (s Store) EnsureSystemFolders(ctx context.Context, userID uuid.UUID) (map[string]uuid.UUID, error) {
	for _, folder := range systemFolders {
		if _, err := s.DB.Exec(ctx, `
			INSERT INTO library_folders (id,user_id,name,system_key)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (user_id,system_key) WHERE system_key IS NOT NULL
			DO UPDATE SET name=EXCLUDED.name,updated_at=now()
		`, uuid.New(), userID, folder.Name, folder.Key); err != nil {
			return nil, err
		}
	}

	rows, err := s.DB.Query(ctx, `SELECT id,system_key FROM library_folders WHERE user_id=$1 AND system_key IS NOT NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]uuid.UUID, len(systemFolders))
	for rows.Next() {
		var id uuid.UUID
		var key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, err
		}
		result[key] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	wantToRead, hasWantToRead := result["want_to_read"]
	reading, hasReading := result["reading"]
	other, hasOther := result["other"]
	if hasWantToRead && hasReading && hasOther {
		if _, err := s.DB.Exec(ctx, `
			UPDATE user_library_items
			SET folder_id=CASE status WHEN 'reading' THEN $2::uuid WHEN 'read' THEN $3::uuid ELSE $4::uuid END
			WHERE user_id=$1 AND folder_id IS NULL
		`, userID, reading, other, wantToRead); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s Store) Folders(ctx context.Context, userID uuid.UUID) ([]FolderOut, error) {
	if _, err := s.EnsureSystemFolders(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(ctx, `
		SELECT f.id,f.name,f.parent_id,f.system_key,f.created_at,count(i.id)
		FROM library_folders f
		LEFT JOIN user_library_items i ON i.user_id=f.user_id AND i.folder_id=f.id
		WHERE f.user_id=$1
		GROUP BY f.id
		ORDER BY CASE f.system_key WHEN 'want_to_read' THEN 0 WHEN 'reading' THEN 1 WHEN 'other' THEN 2 ELSE 3 END, f.created_at, lower(f.name)
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]FolderOut, 0)
	for rows.Next() {
		var folder FolderOut
		if err := rows.Scan(&folder.ID, &folder.Name, &folder.ParentID, &folder.SystemKey, &folder.CreatedAt, &folder.ArticleCount); err != nil {
			return nil, err
		}
		result = append(result, folder)
	}
	return result, rows.Err()
}

func (s Store) CreateFolder(ctx context.Context, userID uuid.UUID, name string, parentID *uuid.UUID) (FolderOut, error) {
	if _, err := s.EnsureSystemFolders(ctx, userID); err != nil {
		return FolderOut{}, err
	}
	if parentID != nil {
		var exists bool
		if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM library_folders WHERE id=$1 AND user_id=$2)`, *parentID, userID).Scan(&exists); err != nil {
			return FolderOut{}, err
		}
		if !exists {
			return FolderOut{}, pgx.ErrNoRows
		}
	}
	folder := FolderOut{ID: uuid.New(), Name: name, ParentID: parentID, CreatedAt: time.Now()}
	err := s.DB.QueryRow(ctx, `INSERT INTO library_folders (id,user_id,parent_id,name) VALUES ($1,$2,$3,$4) RETURNING created_at`, folder.ID, userID, parentID, name).Scan(&folder.CreatedAt)
	return folder, err
}

func (s Store) DeleteFolder(ctx context.Context, userID, folderID uuid.UUID) (bool, error) {
	system, err := s.EnsureSystemFolders(ctx, userID)
	if err != nil {
		return false, err
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var systemKey *string
	if err := tx.QueryRow(ctx, `SELECT system_key FROM library_folders WHERE id=$1 AND user_id=$2 FOR UPDATE`, folderID, userID).Scan(&systemKey); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if systemKey != nil {
		return false, ErrSystemFolderDeletion
	}

	otherID := system["other"]
	if _, err := tx.Exec(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT id FROM library_folders WHERE id=$1 AND user_id=$2
			UNION ALL
			SELECT child.id
			FROM library_folders child
			JOIN subtree parent ON child.parent_id=parent.id
			WHERE child.user_id=$2
		)
		UPDATE user_library_items
		SET folder_id=$3::uuid,status='read'
		WHERE user_id=$2 AND folder_id IN (SELECT id FROM subtree)
	`, folderID, userID, otherID); err != nil {
		return false, err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM library_folders WHERE id=$1 AND user_id=$2 AND system_key IS NULL`, folderID, userID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s Store) Delete(ctx context.Context, userID, paperID uuid.UUID) (bool, error) {
	tag, err := s.DB.Exec(ctx, `DELETE FROM user_library_items WHERE user_id=$1 AND paper_id=$2`, userID, paperID)
	return tag.RowsAffected() > 0, err
}
