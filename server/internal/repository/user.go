package repository

import (
	"context"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByAppleSub(ctx context.Context, sub string) (domain.User, error)
	UpsertByAppleSub(ctx context.Context, sub string) (domain.User, error)
	Update(ctx context.Context, user domain.User) (domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateAPNsToken(ctx context.Context, id uuid.UUID, token string) error
}
