package repository

import (
	"context"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
)

// UserRepository defines persistence operations for users.
//
// Field updates are split into individual methods (UpdateDisplayName,
// UpdateAvatarURL, UpdateAPNsToken) instead of a single "replace the row"
// Update: callers can't accidentally wipe a field by forgetting to repopulate
// it. Pass an empty string to any field update to clear the column.
type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByAppleSub(ctx context.Context, sub string) (domain.User, error)
	UpsertByAppleSub(ctx context.Context, sub string) (domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateDisplayName(ctx context.Context, id uuid.UUID, name string) error
	UpdateAvatarURL(ctx context.Context, id uuid.UUID, url string) error
	UpdateAPNsToken(ctx context.Context, id uuid.UUID, token string) error
}
