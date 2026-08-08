package catalog

import (
	"time"

	"github.com/google/uuid"
)

type AuthorOut struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type PaperVersionOut struct {
	ID           uuid.UUID `json:"id"`
	Source       string    `json:"source"`
	Status       string    `json:"status"`
	PDFKey       *string   `json:"pdf_key"`
	SizeBytes    *int64    `json:"size_bytes"`
	ErrorMessage *string   `json:"error_message"`
}

type PaperOut struct {
	ID            uuid.UUID        `json:"id"`
	Title         string           `json:"title"`
	Abstract      *string          `json:"abstract"`
	Year          *int             `json:"year"`
	Venue         *string          `json:"venue"`
	DOI           *string          `json:"doi"`
	ArxivID       *string          `json:"arxiv_id"`
	Authors       []AuthorOut      `json:"authors"`
	LatestVersion *PaperVersionOut `json:"latest_version"`
	CreatedAt     time.Time        `json:"created_at"`
}

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
