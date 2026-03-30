package reel

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupTestDBOrSkip returns a test DB pool or skips the test if Docker is unavailable.
func setupTestDBOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// SetupTestDB will fatalf if Docker isn't available.
	// We rely on testcontainers' built-in skip when Docker is down.
	return testutil.SetupTestDB(t)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func createTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	u := testutil.CreateUser(t, pool, nil)
	return u.ID
}

func createTestSessionGenerating(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) uuid.UUID {
	t.Helper()
	past := time.Now().Add(-1 * time.Hour)
	s := testutil.CreateSession(t, pool, userID, &testutil.SessionOverrides{
		Deadline: &past,
		Status:   ptr(domain.SessionStatusGenerating),
	})
	return s.ID
}

func createTestSlot(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, order int) uuid.UUID {
	t.Helper()
	s := testutil.CreateSlot(t, pool, sessionID, &testutil.SlotOverrides{
		SlotOrder: &order,
	})
	return s.ID
}

func createTestParticipant(t *testing.T, pool *pgxpool.Pool, sessionID, userID uuid.UUID) {
	t.Helper()
	testutil.CreateParticipant(t, pool, sessionID, userID, nil)
}

func createTestClip(t *testing.T, pool *pgxpool.Pool, sessionID, userID, slotID uuid.UUID) {
	t.Helper()
	slotIDPtr := &slotID
	testutil.CreateClip(t, pool, sessionID, userID, &testutil.ClipOverrides{
		SlotID: &slotIDPtr,
	})
}

func createTestSessionActive(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) uuid.UUID {
	t.Helper()
	past := time.Now().Add(-1 * time.Hour)
	s := testutil.CreateSession(t, pool, userID, &testutil.SessionOverrides{
		Deadline: &past,
		Status:   ptr(domain.SessionStatusActive),
	})
	return s.ID
}

func setRetryCount(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, count int) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "UPDATE sessions SET retry_count = $1 WHERE id = $2", count, sessionID)
	if err != nil {
		t.Fatalf("set retry count: %v", err)
	}
}
