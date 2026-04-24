package repository

import (
	"context"
	"time"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
)

// SessionRepository defines persistence operations for sessions, their slots,
// participants, and per-slot participation records.
//
// AddParticipant serializes joins for a given session using a Postgres
// advisory lock so the 4-participant cap and "one active session per user"
// invariants hold under concurrency.
type SessionRepository interface {
	Create(ctx context.Context, session domain.Session, slots []domain.Slot) (domain.Session, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Session, error)
	GetByInviteToken(ctx context.Context, token string) (domain.Session, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SessionStatus) error
	Cancel(ctx context.Context, id uuid.UUID) error

	AddParticipant(ctx context.Context, sessionID, userID uuid.UUID, displayName string) (domain.Participant, error)
	GetActiveSessionForUser(ctx context.Context, userID uuid.UUID) (*domain.Session, error)

	GetSlots(ctx context.Context, sessionID uuid.UUID) ([]domain.Slot, error)
	UpsertSlotParticipation(ctx context.Context, slotID, userID uuid.UUID, status domain.SlotParticipationStatus) error

	// Scheduler queries.
	GetActiveSessionsPastDeadline(ctx context.Context) ([]domain.Session, error)
	// GetSessionsNeedingReminder returns active sessions whose deadline falls
	// within the given window from now and whose corresponding reminder flag
	// (reminder_2h_sent for 2h, reminder_30m_sent for 30m) is still false.
	// Only 2h and 30m windows are supported.
	GetSessionsNeedingReminder(ctx context.Context, window time.Duration) ([]domain.Session, error)
}
