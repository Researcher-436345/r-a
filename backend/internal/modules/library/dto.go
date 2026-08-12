package library

import (
	"time"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/google/uuid"
)

type ItemOut struct {
	ID       uuid.UUID        `json:"id"`
	Status   string           `json:"status"`
	Favorite bool             `json:"favorite"`
	FolderID *uuid.UUID       `json:"folder_id"`
	AddedAt  time.Time        `json:"added_at"`
	Paper    catalog.PaperOut `json:"paper"`
}

type FolderOut struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	ParentID     *uuid.UUID `json:"parent_id"`
	SystemKey    *string    `json:"system_key"`
	ArticleCount int        `json:"article_count"`
	CreatedAt    time.Time  `json:"created_at"`
}
