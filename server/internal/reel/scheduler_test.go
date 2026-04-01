package reel

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestScheduler_GracefulShutdown(t *testing.T) {
	// Cancel context immediately so Run returns without attempting any DB calls.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	sched := &Scheduler{
		store:    &Store{},
		service:  &Service{},
		interval: time.Second,
		log:      testLogger(),
	}

	err := sched.Run(ctx)
	require.NoError(t, err)
}

func TestScheduler_ClaimsAndProcesses(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionActive(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	clip := createTestClipReturning(t, pool, sess, user, slot)

	seedClipInStorage(t, mem, "clips", clip.S3Key)

	svc := NewService(store, mem, &mockEngine{}, &mockNormalizer{}, "clips", "reels", testLogger())
	sched := &Scheduler{
		store:    store,
		service:  svc,
		interval: time.Second,
		log:      testLogger(),
	}

	// Single poll cycle.
	sched.poll(ctx)

	// Session should be complete.
	var status string
	err := pool.QueryRow(ctx, "SELECT status FROM sessions WHERE id = $1", sess).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "complete", status)
}

func TestScheduler_GenerateFailure_RetriesSession(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionActive(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	createTestClip(t, pool, sess, user, slot)

	// Use failStorage so Generate fails at download.
	fs := &failStorage{}
	svc := NewService(store, fs, &mockEngine{}, &mockNormalizer{}, "clips", "reels", testLogger())
	sched := &Scheduler{
		store:    store,
		service:  svc,
		interval: time.Second,
		log:      testLogger(),
	}

	sched.poll(ctx)

	// Session should be back to active (retryable) with retry_count = 1.
	var status string
	var retryCount int
	err := pool.QueryRow(ctx, "SELECT status, retry_count FROM sessions WHERE id = $1", sess).
		Scan(&status, &retryCount)
	require.NoError(t, err)
	require.Equal(t, "active", status)
	require.Equal(t, 1, retryCount)
}

func TestScheduler_GenerateFailure_ExhaustsRetries(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionActive(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	createTestClip(t, pool, sess, user, slot)

	// Pre-set retry count to MaxRetries-1 so next failure exhausts retries.
	setRetryCount(t, pool, sess, MaxRetries-1)

	fs := &failStorage{}
	svc := NewService(store, fs, &mockEngine{}, &mockNormalizer{}, "clips", "reels", testLogger())
	sched := &Scheduler{
		store:    store,
		service:  svc,
		interval: time.Second,
		log:      testLogger(),
	}

	sched.poll(ctx)

	// Session should be failed.
	var status string
	var retryCount int
	err := pool.QueryRow(ctx, "SELECT status, retry_count FROM sessions WHERE id = $1", sess).
		Scan(&status, &retryCount)
	require.NoError(t, err)

	// FailSession increments and sets to 'failed' when retry_count+1 >= MaxRetries.
	require.Equal(t, "failed", status, fmt.Sprintf("expected failed, got %s (retry_count=%d)", status, retryCount))
	require.Equal(t, MaxRetries, retryCount)
}
