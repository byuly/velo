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

// --- Create ---

func TestCreate_ReturnsToken(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	hash := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Microsecond)

	rt, err := repo.Create(ctx, user.ID, hash, expiresAt)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, rt.ID)
	require.Equal(t, user.ID, rt.UserID)
	require.Equal(t, hash, rt.TokenHash)
	require.WithinDuration(t, expiresAt, rt.ExpiresAt, time.Second)
	require.False(t, rt.CreatedAt.IsZero())
}

// --- GetByHash ---

func TestGetByHash_Found(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	fixture := testutil.CreateRefreshToken(t, pool, user.ID, nil)

	rt, err := repo.GetByHash(ctx, fixture.TokenHash)
	require.NoError(t, err)
	require.Equal(t, fixture.ID, rt.ID)
	require.Equal(t, fixture.UserID, rt.UserID)
	require.Equal(t, fixture.TokenHash, rt.TokenHash)
	require.WithinDuration(t, fixture.ExpiresAt, rt.ExpiresAt, time.Second)
	require.WithinDuration(t, fixture.CreatedAt, rt.CreatedAt, time.Second)
}

func TestGetByHash_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)

	_, err := repo.GetByHash(context.Background(), "nonexistent_hash")
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestGetByHash_ExpiredTokenStillReturned(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	pastExpiry := time.Now().Add(-24 * time.Hour)
	fixture := testutil.CreateRefreshToken(t, pool, user.ID, &testutil.RefreshTokenOverrides{
		ExpiresAt: &pastExpiry,
	})

	rt, err := repo.GetByHash(ctx, fixture.TokenHash)
	require.NoError(t, err)
	require.True(t, rt.ExpiresAt.Before(time.Now()), "expired token should be returned as-is for service to handle")
}

// --- Delete ---

func TestDelete_TokenSucceeds(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	fixture := testutil.CreateRefreshToken(t, pool, user.ID, nil)

	err := repo.Delete(ctx, fixture.ID)
	require.NoError(t, err)

	_, err = repo.GetByHash(ctx, fixture.TokenHash)
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestDelete_TokenNotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)

	err := repo.Delete(context.Background(), uuid.New())
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- DeleteByUserID ---

func TestDeleteByUserID_DeletesAll(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)
	testutil.CreateRefreshToken(t, pool, user.ID, nil)
	testutil.CreateRefreshToken(t, pool, user.ID, nil)

	err := repo.DeleteByUserID(ctx, user.ID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1`, user.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestDeleteByUserID_NoTokens_NoError(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)
	ctx := context.Background()

	user := testutil.CreateUser(t, pool, nil)

	err := repo.DeleteByUserID(ctx, user.ID)
	require.NoError(t, err)
}

func TestDeleteByUserID_OnlyDeletesTargetUser(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewTokenPg(pool)
	ctx := context.Background()

	user1 := testutil.CreateUser(t, pool, nil)
	user2 := testutil.CreateUser(t, pool, nil)
	testutil.CreateRefreshToken(t, pool, user1.ID, nil)
	kept := testutil.CreateRefreshToken(t, pool, user2.ID, nil)

	err := repo.DeleteByUserID(ctx, user1.ID)
	require.NoError(t, err)

	rt, err := repo.GetByHash(ctx, kept.TokenHash)
	require.NoError(t, err)
	require.Equal(t, kept.ID, rt.ID)
}
