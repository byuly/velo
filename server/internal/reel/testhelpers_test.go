package reel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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
	createTestClipReturning(t, pool, sessionID, userID, slotID)
}

func createTestClipReturning(t *testing.T, pool *pgxpool.Pool, sessionID, userID, slotID uuid.UUID) domain.Clip {
	t.Helper()
	slotIDPtr := &slotID
	return testutil.CreateClip(t, pool, sessionID, userID, &testutil.ClipOverrides{
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

// requireFFmpeg skips the test if ffmpeg is not in PATH.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH — skipping integration test")
	}
}

// generateSyntheticClip generates a tiny synthetic MP4 clip using ffmpeg lavfi.
// Returns the path to the generated file. The file lives in dir.
func generateSyntheticClip(t *testing.T, dir, name string, durationSec int) string {
	t.Helper()
	out := filepath.Join(dir, name)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=red:s=720x1280:r=30:d=%d", durationSec),
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:sample_rate=44100:d=%d", durationSec),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "28",
		"-c:a", "aac", "-b:a", "64k", "-ac", "1",
		out,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate synthetic clip %s: %v\n%s", name, err, output)
	}
	return out
}
