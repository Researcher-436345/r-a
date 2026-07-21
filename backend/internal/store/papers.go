package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Papers struct{ DB *pgxpool.Pool }

type Paper struct {
	ID                            uuid.UUID
	Title                         string
	Abstract, Venue, DOI, ArxivID *string
	Year                          *int
	CreatedAt                     time.Time
}
type Version struct {
	ID, PaperID                             uuid.UUID
	VersionNumber                           int
	Source                                  string
	SourceURL, PDFKey, SHA256, ErrorMessage *string
	SizeBytes                               *int64
	Status                                  string
}

func (s Papers) GetPaperOut(ctx context.Context, id uuid.UUID) (models.PaperOut, error) {
	var p models.PaperOut
	err := s.DB.QueryRow(ctx, `SELECT id,title,abstract,year,venue,doi,arxiv_id,created_at FROM papers WHERE id=$1`, id).
		Scan(&p.ID, &p.Title, &p.Abstract, &p.Year, &p.Venue, &p.DOI, &p.ArxivID, &p.CreatedAt)
	if err != nil {
		return p, err
	}
	rows, err := s.DB.Query(ctx, `SELECT a.id,a.name FROM paper_authors pa JOIN authors a ON a.id=pa.author_id WHERE pa.paper_id=$1 ORDER BY pa.position`, id)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var a models.AuthorOut
		if err = rows.Scan(&a.ID, &a.Name); err != nil {
			return p, err
		}
		p.Authors = append(p.Authors, a)
	}
	if err = rows.Err(); err != nil {
		return p, err
	}
	var v models.PaperVersionOut
	err = s.DB.QueryRow(ctx, `SELECT id,source,status,pdf_key,size_bytes,error_message FROM paper_versions WHERE paper_id=$1 ORDER BY version_number DESC LIMIT 1`, id).Scan(&v.ID, &v.Source, &v.Status, &v.PDFKey, &v.SizeBytes, &v.ErrorMessage)
	if err == nil {
		p.LatestVersion = &v
	} else if err != pgx.ErrNoRows {
		return p, err
	}
	return p, nil
}

func (s Papers) EnsureInLibrary(ctx context.Context, userID, paperID uuid.UUID) (bool, error) {
	var ok bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_library_items WHERE user_id=$1 AND paper_id=$2)`, userID, paperID).Scan(&ok)
	return ok, err
}
func (s Papers) AddToLibrary(ctx context.Context, userID, paperID uuid.UUID) (models.LibraryItemOut, error) {
	_, err := s.DB.Exec(ctx, `INSERT INTO user_library_items (id,user_id,paper_id,status,favorite) VALUES($1,$2,$3,'unread',false) ON CONFLICT(user_id,paper_id) DO NOTHING`, uuid.New(), userID, paperID)
	if err != nil {
		return models.LibraryItemOut{}, err
	}
	return s.LibraryItem(ctx, userID, paperID)
}
func (s Papers) FindByArxiv(ctx context.Context, arxivID string) (*Paper, error) {
	return s.findPaper(ctx, `SELECT id,title,abstract,year,venue,doi,arxiv_id,created_at FROM papers WHERE arxiv_id=$1`, arxivID)
}
func (s Papers) FindByDOI(ctx context.Context, doi string) (*Paper, error) {
	return s.findPaper(ctx, `SELECT id,title,abstract,year,venue,doi,arxiv_id,created_at FROM papers WHERE doi=$1`, doi)
}
func (s Papers) findPaper(ctx context.Context, q string, arg string) (*Paper, error) {
	var p Paper
	err := s.DB.QueryRow(ctx, q, arg).Scan(&p.ID, &p.Title, &p.Abstract, &p.Year, &p.Venue, &p.DOI, &p.ArxivID, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &p, err
}
func (s Papers) FindVersionBySHA(ctx context.Context, sha string) (*Version, error) {
	var v Version
	err := s.DB.QueryRow(ctx, `SELECT id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status,error_message FROM paper_versions WHERE sha256=$1 LIMIT 1`, sha).Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Source, &v.SourceURL, &v.PDFKey, &v.SHA256, &v.SizeBytes, &v.Status, &v.ErrorMessage)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &v, err
}
func (s Papers) CreatePaper(ctx context.Context, title string, abstract *string, year *int, venue, doi, arxivID *string) (Paper, error) {
	var p Paper
	err := s.DB.QueryRow(ctx, `INSERT INTO papers(id,title,abstract,year,venue,doi,arxiv_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,title,abstract,year,venue,doi,arxiv_id,created_at`, uuid.New(), title, abstract, year, venue, doi, arxivID).Scan(&p.ID, &p.Title, &p.Abstract, &p.Year, &p.Venue, &p.DOI, &p.ArxivID, &p.CreatedAt)
	return p, err
}
func (s Papers) AttachAuthors(ctx context.Context, paperID uuid.UUID, names []string) error {
	for pos, name := range names {
		normalized := strings.ToLower(strings.Join(strings.Fields(name), " "))
		var id uuid.UUID
		err := s.DB.QueryRow(ctx, `SELECT id FROM authors WHERE normalized_name=$1 LIMIT 1`, normalized).Scan(&id)
		if err == pgx.ErrNoRows {
			err = s.DB.QueryRow(ctx, `INSERT INTO authors(id,name,normalized_name) VALUES($1,$2,$3) RETURNING id`, uuid.New(), strings.TrimSpace(name), normalized).Scan(&id)
		}
		if err != nil {
			return err
		}
		if _, err = s.DB.Exec(ctx, `INSERT INTO paper_authors(id,paper_id,author_id,position) VALUES($1,$2,$3,$4) ON CONFLICT(paper_id,author_id) DO NOTHING`, uuid.New(), paperID, id, pos); err != nil {
			return err
		}
	}
	return nil
}
func (s Papers) CreateVersion(ctx context.Context, paperID uuid.UUID, number int, source string, sourceURL, pdfKey, sha *string, size *int64, status string) (Version, error) {
	var v Version
	err := s.DB.QueryRow(ctx, `INSERT INTO paper_versions(id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status,error_message`, uuid.New(), paperID, number, source, sourceURL, pdfKey, sha, size, status).Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Source, &v.SourceURL, &v.PDFKey, &v.SHA256, &v.SizeBytes, &v.Status, &v.ErrorMessage)
	return v, err
}
func (s Papers) GetVersion(ctx context.Context, id uuid.UUID) (Version, error) {
	var v Version
	err := s.DB.QueryRow(ctx, `SELECT id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status,error_message FROM paper_versions WHERE id=$1`, id).Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Source, &v.SourceURL, &v.PDFKey, &v.SHA256, &v.SizeBytes, &v.Status, &v.ErrorMessage)
	return v, err
}
func (s Papers) UpdateVersion(ctx context.Context, v Version) error {
	_, err := s.DB.Exec(ctx, `UPDATE paper_versions SET pdf_key=$2,sha256=$3,size_bytes=$4,status=$5,error_message=$6 WHERE id=$1`, v.ID, v.PDFKey, v.SHA256, v.SizeBytes, v.Status, v.ErrorMessage)
	return err
}
func (s Papers) LatestVersion(ctx context.Context, paperID uuid.UUID) (Version, error) {
	var v Version
	err := s.DB.QueryRow(ctx, `SELECT id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status,error_message FROM paper_versions WHERE paper_id=$1 ORDER BY version_number DESC LIMIT 1`, paperID).Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Source, &v.SourceURL, &v.PDFKey, &v.SHA256, &v.SizeBytes, &v.Status, &v.ErrorMessage)
	return v, err
}

func (s Papers) ListLibrary(ctx context.Context, userID uuid.UUID, page, limit int, status *string) ([]models.LibraryItemOut, int, error) {
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
	q := `SELECT paper_id FROM user_library_items` + where + ` ORDER BY added_at DESC OFFSET $` + string(rune('0'+len(args)+1)) + ` LIMIT $` + string(rune('0'+len(args)+2))
	args = append(args, (page-1)*limit, limit)
	rows, err := s.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.LibraryItemOut
	for rows.Next() {
		var paperID uuid.UUID
		if err = rows.Scan(&paperID); err != nil {
			return nil, 0, err
		}
		item, err := s.LibraryItem(ctx, userID, paperID)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}
func (s Papers) LibraryItem(ctx context.Context, userID, paperID uuid.UUID) (models.LibraryItemOut, error) {
	var x models.LibraryItemOut
	err := s.DB.QueryRow(ctx, `SELECT id,status,favorite,added_at FROM user_library_items WHERE user_id=$1 AND paper_id=$2`, userID, paperID).Scan(&x.ID, &x.Status, &x.Favorite, &x.AddedAt)
	if err != nil {
		return x, err
	}
	p, err := s.GetPaperOut(ctx, paperID)
	x.Paper = p
	return x, err
}
func (s Papers) PatchLibrary(ctx context.Context, userID, paperID uuid.UUID, status *string, favorite *bool) (models.LibraryItemOut, error) {
	_, err := s.DB.Exec(ctx, `UPDATE user_library_items SET status=COALESCE($3,status),favorite=COALESCE($4,favorite) WHERE user_id=$1 AND paper_id=$2`, userID, paperID, status, favorite)
	if err != nil {
		return models.LibraryItemOut{}, err
	}
	return s.LibraryItem(ctx, userID, paperID)
}
func (s Papers) DeleteLibrary(ctx context.Context, userID, paperID uuid.UUID) (bool, error) {
	tag, err := s.DB.Exec(ctx, `DELETE FROM user_library_items WHERE user_id=$1 AND paper_id=$2`, userID, paperID)
	return tag.RowsAffected() > 0, err
}

func (s Papers) ListAnnotations(ctx context.Context, userID, paperID uuid.UUID) ([]models.AnnotationOut, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,paper_id,page,rect,selected_text,note,color,created_at,updated_at FROM annotations WHERE user_id=$1 AND paper_id=$2 ORDER BY page,created_at`, userID, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnnotations(rows)
}
func (s Papers) CreateAnnotation(ctx context.Context, userID, paperID uuid.UUID, page int, rect *models.AnnotationRect, text, note, color string) (models.AnnotationOut, error) {
	var raw []byte
	if rect != nil {
		raw, _ = json.Marshal(rect)
	}
	return s.scanAnnotation(ctx, `INSERT INTO annotations(id,paper_id,user_id,page,rect,selected_text,note,color) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,paper_id,page,rect,selected_text,note,color,created_at,updated_at`, uuid.New(), paperID, userID, page, raw, text, note, color)
}
func (s Papers) PatchAnnotation(ctx context.Context, userID, id uuid.UUID, note string) (models.AnnotationOut, error) {
	return s.scanAnnotation(ctx, `UPDATE annotations SET note=$3,updated_at=now() WHERE id=$1 AND user_id=$2 RETURNING id,paper_id,page,rect,selected_text,note,color,created_at,updated_at`, id, userID, note)
}
func (s Papers) DeleteAnnotation(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	tag, err := s.DB.Exec(ctx, `DELETE FROM annotations WHERE id=$1 AND user_id=$2`, id, userID)
	return tag.RowsAffected() > 0, err
}
func (s Papers) scanAnnotation(ctx context.Context, q string, args ...any) (models.AnnotationOut, error) {
	var a models.AnnotationOut
	var raw []byte
	err := s.DB.QueryRow(ctx, q, args...).Scan(&a.ID, &a.PaperID, &a.Page, &raw, &a.SelectedText, &a.Note, &a.Color, &a.CreatedAt, &a.UpdatedAt)
	a.Rect = models.RectFromJSON(raw)
	return a, err
}
func scanAnnotations(rows pgx.Rows) ([]models.AnnotationOut, error) {
	var out []models.AnnotationOut
	for rows.Next() {
		var a models.AnnotationOut
		var raw []byte
		if err := rows.Scan(&a.ID, &a.PaperID, &a.Page, &raw, &a.SelectedText, &a.Note, &a.Color, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Rect = models.RectFromJSON(raw)
		out = append(out, a)
	}
	return out, rows.Err()
}
