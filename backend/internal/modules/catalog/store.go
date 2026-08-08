package catalog

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

func (s Store) GetPaperOut(ctx context.Context, id uuid.UUID) (PaperOut, error) {
	var p PaperOut
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
	p.Authors = make([]AuthorOut, 0)
	for rows.Next() {
		var a AuthorOut
		if err = rows.Scan(&a.ID, &a.Name); err != nil {
			return p, err
		}
		p.Authors = append(p.Authors, a)
	}
	if err = rows.Err(); err != nil {
		return p, err
	}
	var v PaperVersionOut
	err = s.DB.QueryRow(ctx, `SELECT id,source,status,pdf_key,size_bytes,error_message FROM paper_versions WHERE paper_id=$1 ORDER BY version_number DESC LIMIT 1`, id).Scan(&v.ID, &v.Source, &v.Status, &v.PDFKey, &v.SizeBytes, &v.ErrorMessage)
	if err == nil {
		p.LatestVersion = &v
	} else if err != pgx.ErrNoRows {
		return p, err
	}
	return p, nil
}

func (s Store) FindByArxiv(ctx context.Context, arxivID string) (*Paper, error) {
	return s.findPaper(ctx, `SELECT id,title,abstract,year,venue,doi,arxiv_id,created_at FROM papers WHERE arxiv_id=$1`, arxivID)
}
func (s Store) FindByDOI(ctx context.Context, doi string) (*Paper, error) {
	return s.findPaper(ctx, `SELECT id,title,abstract,year,venue,doi,arxiv_id,created_at FROM papers WHERE doi=$1`, doi)
}
func (s Store) findPaper(ctx context.Context, q string, arg string) (*Paper, error) {
	var p Paper
	err := s.DB.QueryRow(ctx, q, arg).Scan(&p.ID, &p.Title, &p.Abstract, &p.Year, &p.Venue, &p.DOI, &p.ArxivID, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &p, err
}
func (s Store) FindVersionBySHA(ctx context.Context, sha string) (*Version, error) {
	var v Version
	err := s.DB.QueryRow(ctx, `SELECT id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status,error_message FROM paper_versions WHERE sha256=$1 LIMIT 1`, sha).Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Source, &v.SourceURL, &v.PDFKey, &v.SHA256, &v.SizeBytes, &v.Status, &v.ErrorMessage)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &v, err
}
func (s Store) CreatePaper(ctx context.Context, title string, abstract *string, year *int, venue, doi, arxivID *string) (Paper, error) {
	var p Paper
	err := s.DB.QueryRow(ctx, `INSERT INTO papers(id,title,abstract,year,venue,doi,arxiv_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,title,abstract,year,venue,doi,arxiv_id,created_at`, uuid.New(), title, abstract, year, venue, doi, arxivID).Scan(&p.ID, &p.Title, &p.Abstract, &p.Year, &p.Venue, &p.DOI, &p.ArxivID, &p.CreatedAt)
	return p, err
}

func (s Store) UpdatePaperTitle(ctx context.Context, id uuid.UUID, title string) error {
	_, err := s.DB.Exec(ctx, `UPDATE papers SET title=$2 WHERE id=$1`, id, title)
	return err
}

func (s Store) UpdatePaperMeta(ctx context.Context, id uuid.UUID, title string, abstract *string, year *int, venue, doi, arxivID *string) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE papers SET
			title=$2,
			abstract=COALESCE($3, abstract),
			year=COALESCE($4, year),
			venue=COALESCE($5, venue),
			doi=COALESCE($6, doi),
			arxiv_id=COALESCE($7, arxiv_id)
		WHERE id=$1`, id, title, abstract, year, venue, doi, arxivID)
	return err
}
func (s Store) AttachAuthors(ctx context.Context, paperID uuid.UUID, names []string) error {
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
func (s Store) CreateVersion(ctx context.Context, paperID uuid.UUID, number int, source string, sourceURL, pdfKey, sha *string, size *int64, status string) (Version, error) {
	var v Version
	err := s.DB.QueryRow(ctx, `INSERT INTO paper_versions(id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status,error_message`, uuid.New(), paperID, number, source, sourceURL, pdfKey, sha, size, status).Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Source, &v.SourceURL, &v.PDFKey, &v.SHA256, &v.SizeBytes, &v.Status, &v.ErrorMessage)
	return v, err
}
func (s Store) GetVersion(ctx context.Context, id uuid.UUID) (Version, error) {
	var v Version
	err := s.DB.QueryRow(ctx, `SELECT id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status,error_message FROM paper_versions WHERE id=$1`, id).Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Source, &v.SourceURL, &v.PDFKey, &v.SHA256, &v.SizeBytes, &v.Status, &v.ErrorMessage)
	return v, err
}
func (s Store) UpdateVersion(ctx context.Context, v Version) error {
	_, err := s.DB.Exec(ctx, `UPDATE paper_versions SET pdf_key=$2,sha256=$3,size_bytes=$4,status=$5,error_message=$6 WHERE id=$1`, v.ID, v.PDFKey, v.SHA256, v.SizeBytes, v.Status, v.ErrorMessage)
	return err
}
func (s Store) LatestVersion(ctx context.Context, paperID uuid.UUID) (Version, error) {
	var v Version
	err := s.DB.QueryRow(ctx, `SELECT id,paper_id,version_number,source,source_url,pdf_key,sha256,size_bytes,status,error_message FROM paper_versions WHERE paper_id=$1 ORDER BY version_number DESC LIMIT 1`, paperID).Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Source, &v.SourceURL, &v.PDFKey, &v.SHA256, &v.SizeBytes, &v.Status, &v.ErrorMessage)
	return v, err
}
