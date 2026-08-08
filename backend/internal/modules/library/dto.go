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
	AddedAt  time.Time        `json:"added_at"`
	Paper    catalog.PaperOut `json:"paper"`
}
