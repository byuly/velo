package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/repository"
	"github.com/byuly/velo/server/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestClipCreate_Succeeds(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, user.ID, nil)
	slot := testutil.CreateSlot(t, pool, session.ID, nil)

	now := time.Now().UTC().Truncate(time.Microsecond)
	in := domain.Clip{
		SessionID:         session.ID,
		UserID:            &user.ID,
		SlotID:            &slot.ID,
		S3Key:             "clips/" + uuid.New().String() + ".m4a",
		RecordedAt:        now,
		ArrivedAt:         now.Add(2 * time.Second),
		RecordedAtClamped: true,
		DurationMs:        4200,
	}

	out, err := repo.Create(ctx, in)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, out.ID)
	require.False(t, out.CreatedAt.IsZero())
	require.Equal(t, in.SessionID, out.SessionID)
	require.Equal(t, in.UserID, out.UserID)
	require.Equal(t, in.SlotID, out.SlotID)
	require.Equal(t, in.S3Key, out.S3Key)
	require.True(t, in.RecordedAt.Equal(out.RecordedAt))
	require.True(t, in.ArrivedAt.Equal(out.ArrivedAt))
	require.True(t, out.RecordedAtClamped)
	require.Equal(t, 4200, out.DurationMs)
}

func TestClipCreate_DuplicateS3Key(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, user.ID, nil)
	key := "clips/dup-" + uuid.New().String() + ".m4a"

	testutil.CreateClip(t, pool, session.ID, user.ID, &testutil.ClipOverrides{
		S3Key: testutil.Ptr(key),
	})

	_, err := repo.Create(ctx, domain.Clip{
		SessionID:  session.ID,
		UserID:     &user.ID,
		S3Key:      key,
		RecordedAt: time.Now(),
		ArrivedAt:  time.Now(),
		DurationMs: 1000,
	})
	require.True(t, errors.Is(err, domain.ErrDuplicateClip), "expected ErrDuplicateClip, got %v", err)
}

func TestClipGetByID_Found(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, user.ID, nil)
	fixture := testutil.CreateClip(t, pool, session.ID, user.ID, nil)

	out, err := repo.GetByID(ctx, fixture.ID)
	require.NoError(t, err)
	require.Equal(t, fixture.ID, out.ID)
	require.Equal(t, fixture.S3Key, out.S3Key)
	require.Equal(t, fixture.DurationMs, out.DurationMs)
}

func TestClipGetByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)

	_, err := repo.GetByID(context.Background(), uuid.New())
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestClipGetBySessionID_ReturnsOrdered(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, user.ID, nil)

	base := time.Now().UTC().Add(-1 * time.Hour)
	t1 := base
	t2 := base.Add(1 * time.Minute)
	t3 := base.Add(2 * time.Minute)

	// insert out of order
	testutil.CreateClip(t, pool, session.ID, user.ID, &testutil.ClipOverrides{RecordedAt: &t2})
	testutil.CreateClip(t, pool, session.ID, user.ID, &testutil.ClipOverrides{RecordedAt: &t3})
	testutil.CreateClip(t, pool, session.ID, user.ID, &testutil.ClipOverrides{RecordedAt: &t1})

	clips, err := repo.GetBySessionID(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, clips, 3)
	require.True(t, clips[0].RecordedAt.Equal(t1))
	require.True(t, clips[1].RecordedAt.Equal(t2))
	require.True(t, clips[2].RecordedAt.Equal(t3))
}

func TestClipGetBySessionID_EmptyForUnknownSession(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)

	clips, err := repo.GetBySessionID(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Empty(t, clips)
}

func TestClipGetBySessionAndUser_FiltersByUser(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)
	ctx := context.Background()

	userA := testutil.CreateUser(t, pool, nil)
	userB := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, userA.ID, nil)

	testutil.CreateClip(t, pool, session.ID, userA.ID, nil)
	testutil.CreateClip(t, pool, session.ID, userA.ID, nil)
	testutil.CreateClip(t, pool, session.ID, userB.ID, nil)
	testutil.CreateClip(t, pool, session.ID, userB.ID, nil)

	got, err := repo.GetBySessionAndUser(ctx, session.ID, userA.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, c := range got {
		require.NotNil(t, c.UserID)
		require.Equal(t, userA.ID, *c.UserID)
	}
}

func TestClipGetTotalDurationForSlot_Sums(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, user.ID, nil)
	slot := testutil.CreateSlot(t, pool, session.ID, nil)
	slotPtr := &slot.ID

	for _, d := range []int{1000, 2000, 3000} {
		dur := d
		testutil.CreateClip(t, pool, session.ID, user.ID, &testutil.ClipOverrides{
			SlotID:     &slotPtr,
			DurationMs: &dur,
		})
	}

	total, err := repo.GetTotalDurationForSlot(ctx, slot.ID)
	require.NoError(t, err)
	require.Equal(t, 6000, total)
}

func TestClipGetTotalDurationForSlot_ZeroForEmpty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewClipPg(pool)

	total, err := repo.GetTotalDurationForSlot(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, 0, total)
}
