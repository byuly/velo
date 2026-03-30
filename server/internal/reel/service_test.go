package reel

import (
	"context"
	"fmt"
	"testing"

	"github.com/byuly/velo/server/internal/storage"
	"github.com/stretchr/testify/require"
)

// failStorage is a Storage that fails on Download.
type failStorage struct {
	storage.MemStorage
}

func (f *failStorage) Download(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("simulated S3 failure")
}

func TestGenerate_ZeroSubmitters(t *testing.T) {
	// This test doesn't need FFmpeg or S3 — it exercises the zero-submitter path.
	// It needs a real DB though, which requires Docker. Skip if unavailable.
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)

	// Session has no clips → zero submitters.
	createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)

	svc := NewService(store, mem, nil, nil, "clips", "reels", testLogger())
	err := svc.Generate(ctx, sess)
	require.NoError(t, err)

	// Session should be complete with no reel URL.
	var status string
	var reelURL *string
	err = pool.QueryRow(ctx, "SELECT status, reel_url FROM sessions WHERE id = $1", sess).
		Scan(&status, &reelURL)
	require.NoError(t, err)
	require.Equal(t, "complete", status)
	require.Nil(t, reelURL)
}

func TestGenerate_DownloadFailure(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	fs := &failStorage{}
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	createTestClip(t, pool, sess, user, slot)

	svc := NewService(store, fs, nil, nil, "clips", "reels", testLogger())
	err := svc.Generate(ctx, sess)
	require.Error(t, err)
	require.Contains(t, err.Error(), "download clip")
}
