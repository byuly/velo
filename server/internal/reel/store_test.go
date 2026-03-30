package reel

import (
	"context"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestClaimDueSessions_ClaimsActiveSessionsPastDeadline(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	past := time.Now().Add(-1 * time.Hour)

	// Active session with deadline in the past → should be claimed.
	s := testutil.CreateSession(t, pool, user.ID, &testutil.SessionOverrides{
		Deadline: &past,
		Status:   ptr(domain.SessionStatusActive),
	})

	ids, err := store.ClaimDueSessions(ctx, 10)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	require.Equal(t, s.ID, ids[0])

	// Verify it's now 'generating'.
	var status string
	err = pool.QueryRow(ctx, "SELECT status FROM sessions WHERE id = $1", s.ID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "generating", status)
}

func TestClaimDueSessions_SkipsFutureDeadline(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	future := time.Now().Add(24 * time.Hour)

	testutil.CreateSession(t, pool, user.ID, &testutil.SessionOverrides{
		Deadline: &future,
		Status:   ptr(domain.SessionStatusActive),
	})

	ids, err := store.ClaimDueSessions(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, ids)
}

func TestClaimDueSessions_SkipsRetryExhausted(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	past := time.Now().Add(-1 * time.Hour)

	s := testutil.CreateSession(t, pool, user.ID, &testutil.SessionOverrides{
		Deadline: &past,
		Status:   ptr(domain.SessionStatusActive),
	})

	// Set retry_count to MaxRetries.
	_, err := pool.Exec(ctx, "UPDATE sessions SET retry_count = $1 WHERE id = $2", MaxRetries, s.ID)
	require.NoError(t, err)

	ids, err := store.ClaimDueSessions(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, ids)
}

func TestClaimDueSessions_SkipsNonActiveStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	past := time.Now().Add(-1 * time.Hour)

	for _, status := range []domain.SessionStatus{
		domain.SessionStatusGenerating,
		domain.SessionStatusComplete,
		domain.SessionStatusFailed,
		domain.SessionStatusCancelled,
	} {
		testutil.CreateSession(t, pool, user.ID, &testutil.SessionOverrides{
			Deadline: &past,
			Status:   ptr(status),
		})
	}

	ids, err := store.ClaimDueSessions(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, ids)
}

func TestFetchSessionData_ReturnsAllData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	store := NewStore(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	other := testutil.CreateUser(t, pool, nil)

	sess := testutil.CreateSession(t, pool, creator.ID, nil)
	slot := testutil.CreateSlot(t, pool, sess.ID, &testutil.SlotOverrides{
		SlotOrder: intPtr(0),
	})

	testutil.CreateParticipant(t, pool, sess.ID, creator.ID, &testutil.ParticipantOverrides{
		DisplayNameSnapshot: strPtr("Creator"),
	})
	testutil.CreateParticipant(t, pool, sess.ID, other.ID, &testutil.ParticipantOverrides{
		DisplayNameSnapshot: strPtr("Other"),
	})

	slotID := slot.ID
	slotIDPtr := &slotID
	testutil.CreateClip(t, pool, sess.ID, creator.ID, &testutil.ClipOverrides{
		SlotID: &slotIDPtr,
	})

	title := "sleeping"
	titlePtr := &title
	testutil.CreateSlotParticipation(t, pool, slot.ID, creator.ID, &testutil.SlotParticipationOverrides{
		Title: &titlePtr,
	})

	data, err := store.FetchSessionData(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, sess.ID, data.Session.ID)
	require.Equal(t, sess.MaxSectionDurationS, data.Session.MaxSectionDurationS)
	require.Len(t, data.Slots, 1)
	require.Equal(t, 0, data.Slots[0].SlotOrder)
	require.Len(t, data.Participants, 2)
	require.Len(t, data.Clips, 1)
	require.Len(t, data.Participations, 1)
	require.Equal(t, "sleeping", *data.Participations[0].Title)
}

func TestCompleteSession_SetsStatusAndReelURL(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	sess := testutil.CreateSession(t, pool, user.ID, &testutil.SessionOverrides{
		Status: ptr(domain.SessionStatusGenerating),
	})

	err := store.CompleteSession(ctx, sess.ID, "https://cdn.test/reel.mp4")
	require.NoError(t, err)

	var status string
	var reelURL *string
	var completedAt *time.Time
	err = pool.QueryRow(ctx, "SELECT status, reel_url, completed_at FROM sessions WHERE id = $1", sess.ID).
		Scan(&status, &reelURL, &completedAt)
	require.NoError(t, err)
	require.Equal(t, "complete", status)
	require.NotNil(t, reelURL)
	require.Equal(t, "https://cdn.test/reel.mp4", *reelURL)
	require.NotNil(t, completedAt)
}

func TestCompleteSession_EmptyReelURL(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	sess := testutil.CreateSession(t, pool, user.ID, &testutil.SessionOverrides{
		Status: ptr(domain.SessionStatusGenerating),
	})

	err := store.CompleteSession(ctx, sess.ID, "")
	require.NoError(t, err)

	var status string
	var reelURL *string
	err = pool.QueryRow(ctx, "SELECT status, reel_url FROM sessions WHERE id = $1", sess.ID).
		Scan(&status, &reelURL)
	require.NoError(t, err)
	require.Equal(t, "complete", status)
	require.Nil(t, reelURL)
}

func TestFailSession_RetriesAndExhausts(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	store := NewStore(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	sess := testutil.CreateSession(t, pool, user.ID, &testutil.SessionOverrides{
		Status: ptr(domain.SessionStatusGenerating),
	})

	// First failure: should go back to active.
	require.NoError(t, store.FailSession(ctx, sess.ID))
	var status string
	var retryCount int
	err := pool.QueryRow(ctx, "SELECT status, retry_count FROM sessions WHERE id = $1", sess.ID).
		Scan(&status, &retryCount)
	require.NoError(t, err)
	require.Equal(t, "active", status)
	require.Equal(t, 1, retryCount)

	// Set to generating again for second failure.
	_, err = pool.Exec(ctx, "UPDATE sessions SET status = 'generating' WHERE id = $1", sess.ID)
	require.NoError(t, err)
	require.NoError(t, store.FailSession(ctx, sess.ID))
	err = pool.QueryRow(ctx, "SELECT status, retry_count FROM sessions WHERE id = $1", sess.ID).
		Scan(&status, &retryCount)
	require.NoError(t, err)
	require.Equal(t, "active", status)
	require.Equal(t, 2, retryCount)

	// Set to generating again for third (final) failure.
	_, err = pool.Exec(ctx, "UPDATE sessions SET status = 'generating' WHERE id = $1", sess.ID)
	require.NoError(t, err)
	require.NoError(t, store.FailSession(ctx, sess.ID))
	err = pool.QueryRow(ctx, "SELECT status, retry_count FROM sessions WHERE id = $1", sess.ID).
		Scan(&status, &retryCount)
	require.NoError(t, err)
	require.Equal(t, "failed", status)
	require.Equal(t, 3, retryCount)
}

// Helpers

func ptr[T any](v T) *T       { return &v }
func strPtr(s string) *string  { return &s }
func intPtr(i int) *int        { return &i }
