package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/repository"
	"github.com/byuly/velo/server/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// --- GetByID ---

func TestGetByID_Found(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		DisplayName: ptr(ptr("Alice")),
		AvatarURL:   ptr(ptr("https://example.com/avatar.png")),
	})

	u, err := repo.GetByID(ctx, fixture.ID)
	require.NoError(t, err)
	require.Equal(t, fixture.ID, u.ID)
	require.Equal(t, fixture.AppleSub, u.AppleSub)
	require.Equal(t, fixture.DisplayName, u.DisplayName)
	require.Equal(t, fixture.AvatarURL, u.AvatarURL)
	require.Equal(t, fixture.APNsToken, u.APNsToken)
	require.WithinDuration(t, fixture.CreatedAt, u.CreatedAt, 0)
	require.WithinDuration(t, fixture.UpdatedAt, u.UpdatedAt, 0)
}

func TestGetByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)

	_, err := repo.GetByID(context.Background(), uuid.New())
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- GetByAppleSub ---

func TestGetByAppleSub_Found(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		AppleSub: ptr("sub_abc_123"),
	})

	u, err := repo.GetByAppleSub(ctx, "sub_abc_123")
	require.NoError(t, err)
	require.Equal(t, fixture.ID, u.ID)
	require.Equal(t, "sub_abc_123", u.AppleSub)
}

func TestGetByAppleSub_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)

	_, err := repo.GetByAppleSub(context.Background(), "nonexistent_sub")
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- UpsertByAppleSub ---

func TestUpsertByAppleSub_NewUser(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	u, err := repo.UpsertByAppleSub(ctx, "brand_new_sub")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, u.ID)
	require.Equal(t, "brand_new_sub", u.AppleSub)
	require.Nil(t, u.DisplayName)
	require.Nil(t, u.AvatarURL)
	require.Nil(t, u.APNsToken)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE apple_sub = $1`, "brand_new_sub").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestUpsertByAppleSub_ExistingUser(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		AppleSub: ptr("existing_sub"),
	})

	u1, err := repo.UpsertByAppleSub(ctx, "existing_sub")
	require.NoError(t, err)
	require.Equal(t, fixture.ID, u1.ID)

	u2, err := repo.UpsertByAppleSub(ctx, "existing_sub")
	require.NoError(t, err)
	require.Equal(t, fixture.ID, u2.ID)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE apple_sub = $1`, "existing_sub").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestUpsertByAppleSub_DoesNotTouchUpdatedAt(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		AppleSub: ptr("stable_sub"),
	})

	u, err := repo.UpsertByAppleSub(ctx, "stable_sub")
	require.NoError(t, err)
	require.True(t, fixture.UpdatedAt.Equal(u.UpdatedAt), "upsert on existing user must not change updated_at")
}

// --- Update ---

func TestUpdate_DisplayName(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, nil)

	updated, err := repo.Update(ctx, domain.User{ID: fixture.ID, DisplayName: ptr("Alice"), AvatarURL: nil})
	require.NoError(t, err)
	require.NotNil(t, updated.DisplayName)
	require.Equal(t, "Alice", *updated.DisplayName)
	require.Nil(t, updated.AvatarURL)
	require.True(t, updated.UpdatedAt.After(fixture.UpdatedAt) || updated.UpdatedAt.Equal(fixture.UpdatedAt))
	// APNsToken unchanged
	require.Equal(t, fixture.APNsToken, updated.APNsToken)
}

func TestUpdate_ClearDisplayName(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		DisplayName: ptr(ptr("Bob")),
	})

	updated, err := repo.Update(ctx, domain.User{ID: fixture.ID, DisplayName: nil, AvatarURL: nil})
	require.NoError(t, err)
	require.Nil(t, updated.DisplayName)
}

func TestUpdate_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)

	_, err := repo.Update(context.Background(), domain.User{ID: uuid.New()})
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- Delete ---

func TestDelete_Succeeds(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, nil)

	err := repo.Delete(ctx, fixture.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, fixture.ID)
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestDelete_CascadesRefreshTokens(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, nil)
	testutil.CreateRefreshToken(t, pool, fixture.ID, nil)

	err := repo.Delete(ctx, fixture.ID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1`, fixture.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestDelete_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)

	err := repo.Delete(context.Background(), uuid.New())
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- UpdateAPNsToken ---

func TestUpdateAPNsToken_Set(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, nil)

	err := repo.UpdateAPNsToken(ctx, fixture.ID, "token_abc")
	require.NoError(t, err)

	u, err := repo.GetByID(ctx, fixture.ID)
	require.NoError(t, err)
	require.NotNil(t, u.APNsToken)
	require.Equal(t, "token_abc", *u.APNsToken)
}

func TestUpdateAPNsToken_ClearWithEmptyString(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		APNsToken: ptr(ptr("old_token")),
	})

	err := repo.UpdateAPNsToken(ctx, fixture.ID, "")
	require.NoError(t, err)

	u, err := repo.GetByID(ctx, fixture.ID)
	require.NoError(t, err)
	require.Nil(t, u.APNsToken)
}

func TestUpdateAPNsToken_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)

	err := repo.UpdateAPNsToken(context.Background(), uuid.New(), "some_token")
	require.True(t, errors.Is(err, domain.ErrNotFound))
}
