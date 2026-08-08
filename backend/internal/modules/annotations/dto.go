package annotations

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Out struct {
	ID           uuid.UUID `json:"id"`
	PaperID      uuid.UUID `json:"paper_id"`
	Page         int       `json:"page"`
	Rect         *Rect     `json:"rect"`
	SelectedText string    `json:"selected_text"`
	Note         string    `json:"note"`
	Color        string    `json:"color"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func RectFromJSON(raw []byte) *Rect {
	if len(raw) == 0 {
		return nil
	}
	var r Rect
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil
	}
	return &r
}
