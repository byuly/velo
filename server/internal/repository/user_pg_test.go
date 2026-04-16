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

// --- UpdateDisplayName ---

func TestUpdateDisplayName_Set(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		AvatarURL: ptr(ptr("https://example.com/a.png")),
		APNsToken: ptr(ptr("apns_token_xyz")),
	})

	err := repo.UpdateDisplayName(ctx, fixture.ID, "Alice")
	require.NoError(t, err)

	u, err := repo.GetByID(ctx, fixture.ID)
	require.NoError(t, err)
	require.NotNil(t, u.DisplayName)
	require.Equal(t, "Alice", *u.DisplayName)
	// Sibling columns untouched
	require.Equal(t, fixture.AvatarURL, u.AvatarURL)
	require.Equal(t, fixture.APNsToken, u.APNsToken)
	require.True(t, u.UpdatedAt.After(fixture.UpdatedAt) || u.UpdatedAt.Equal(fixture.UpdatedAt))
}

func TestUpdateDisplayName_ClearWithEmptyString(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		DisplayName: ptr(ptr("Bob")),
	})

	err := repo.UpdateDisplayName(ctx, fixture.ID, "")
	require.NoError(t, err)

	u, err := repo.GetByID(ctx, fixture.ID)
	require.NoError(t, err)
	require.Nil(t, u.DisplayName)
}

func TestUpdateDisplayName_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)

	err := repo.UpdateDisplayName(context.Background(), uuid.New(), "Alice")
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- UpdateAvatarURL ---

func TestUpdateAvatarURL_Set(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		DisplayName: ptr(ptr("Carol")),
	})

	err := repo.UpdateAvatarURL(ctx, fixture.ID, "https://example.com/avatar.png")
	require.NoError(t, err)

	u, err := repo.GetByID(ctx, fixture.ID)
	require.NoError(t, err)
	require.NotNil(t, u.AvatarURL)
	require.Equal(t, "https://example.com/avatar.png", *u.AvatarURL)
	// DisplayName untouched
	require.Equal(t, fixture.DisplayName, u.DisplayName)
}

func TestUpdateAvatarURL_ClearWithEmptyString(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)
	ctx := context.Background()

	fixture := testutil.CreateUser(t, pool, &testutil.UserOverrides{
		AvatarURL: ptr(ptr("https://example.com/old.png")),
	})

	err := repo.UpdateAvatarURL(ctx, fixture.ID, "")
	require.NoError(t, err)

	u, err := repo.GetByID(ctx, fixture.ID)
	require.NoError(t, err)
	require.Nil(t, u.AvatarURL)
}

func TestUpdateAvatarURL_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewUserPg(pool)

	err := repo.UpdateAvatarURL(context.Background(), uuid.New(), "https://example.com/x.png")
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
