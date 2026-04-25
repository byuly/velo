package repository

import (
	"context"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
)

// ClipRepository persists clip records uploaded by participants.
//
// Create returns domain.ErrDuplicateClip if a clip with the same s3_key
// already exists; the s3_key UNIQUE constraint provides idempotency for
// retried confirmations.
type ClipRepository interface {
	Create(ctx context.Context, clip domain.Clip) (domain.Clip, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Clip, error)
	GetBySessionID(ctx context.Context, sessionID uuid.UUID) ([]domain.Clip, error)
	GetBySessionAndUser(ctx context.Context, sessionID, userID uuid.UUID) ([]domain.Clip, error)
	GetTotalDurationForSlot(ctx context.Context, slotID uuid.UUID) (int, error)
}
