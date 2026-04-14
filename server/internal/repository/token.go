package repository

import (
	"context"
	"time"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
)

// TokenRepository defines persistence operations for refresh tokens.
type TokenRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (domain.RefreshToken, error)
	GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
}
