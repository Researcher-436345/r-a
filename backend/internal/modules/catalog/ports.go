package catalog

import (
	"context"

	"github.com/google/uuid"
)

// Membership is implemented by the library module (wired in app).
// Catalog must not import library directly.
type Membership interface {
	Add(ctx context.Context, userID, paperID uuid.UUID) error
	Has(ctx context.Context, userID, paperID uuid.UUID) (bool, error)
}
