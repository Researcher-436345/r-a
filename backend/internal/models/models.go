package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

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

type LibraryItemOut struct {
	ID       uuid.UUID `json:"id"`
	Status   string    `json:"status"`
	Favorite bool      `json:"favorite"`
	AddedAt  time.Time `json:"added_at"`
	Paper    PaperOut  `json:"paper"`
}

type AnnotationRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type AnnotationOut struct {
	ID           uuid.UUID       `json:"id"`
	PaperID      uuid.UUID       `json:"paper_id"`
	Page         int             `json:"page"`
	Rect         *AnnotationRect `json:"rect"`
	SelectedText string          `json:"selected_text"`
	Note         string          `json:"note"`
	Color        string          `json:"color"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type TrendingPaper struct {
	ArxivID         string   `json:"arxiv_id"`
	Title           string   `json:"title"`
	Abstract        *string  `json:"abstract"`
	Authors         []string `json:"authors"`
	PublishedAt     string   `json:"published_at"`
	Category        string   `json:"category"`
	PopularityScore float64  `json:"popularity_score"`
	PDFURL          string   `json:"pdf_url"`
	AbsURL          string   `json:"abs_url"`
}

func RectFromJSON(raw []byte) *AnnotationRect {
	if len(raw) == 0 {
		return nil
	}
	var r AnnotationRect
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil
	}
	return &r
}
