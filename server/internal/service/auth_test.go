package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/repository"
	"github.com/byuly/velo/server/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type mockApple struct {
	sub string
	err error
}

func (m *mockApple) Validate(_ context.Context, _ string) (string, error) {
	return m.sub, m.err
}

type mockUserRepo struct {
	upsertResult domain.User
	upsertErr    error
}

func (m *mockUserRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.User, error) {
	panic("not implemented")
}
func (m *mockUserRepo) GetByAppleSub(_ context.Context, _ string) (domain.User, error) {
	panic("not implemented")
}
func (m *mockUserRepo) UpsertByAppleSub(_ context.Context, _ string) (domain.User, error) {
	return m.upsertResult, m.upsertErr
}
func (m *mockUserRepo) Delete(_ context.Context, _ uuid.UUID) error { panic("not implemented") }
func (m *mockUserRepo) UpdateDisplayName(_ context.Context, _ uuid.UUID, _ string) error {
	panic("not implemented")
}
func (m *mockUserRepo) UpdateAvatarURL(_ context.Context, _ uuid.UUID, _ string) error {
	panic("not implemented")
}
func (m *mockUserRepo) UpdateAPNsToken(_ context.Context, _ uuid.UUID, _ string) error {
	panic("not implemented")
}

var _ repository.UserRepository = (*mockUserRepo)(nil)

type mockTokenRepo struct {
	createResult domain.RefreshToken
	createErr    error
	capturedHash string

	getResult domain.RefreshToken
	getErr    error

	deleteByUserIDErr    error
	deleteByUserIDCalled bool
}

func (m *mockTokenRepo) Create(_ context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (domain.RefreshToken, error) {
	m.capturedHash = tokenHash
	if m.createErr != nil {
		return domain.RefreshToken{}, m.createErr
	}
	rt := m.createResult
	rt.UserID = userID
	rt.TokenHash = tokenHash
	rt.ExpiresAt = expiresAt
	return rt, nil
}
func (m *mockTokenRepo) GetByHash(_ context.Context, hash string) (domain.RefreshToken, error) {
	return m.getResult, m.getErr
}
func (m *mockTokenRepo) Delete(_ context.Context, _ uuid.UUID) error { panic("not implemented") }
func (m *mockTokenRepo) DeleteByUserID(_ context.Context, _ uuid.UUID) error {
	m.deleteByUserIDCalled = true
	return m.deleteByUserIDErr
}

var _ repository.TokenRepository = (*mockTokenRepo)(nil)

type mockBlocklist struct {
	blocked map[string]bool
	err     error
}

func (m *mockBlocklist) Block(_ context.Context, jti string, _ time.Duration) error {
	if m.err != nil {
		return m.err
	}
	m.blocked[jti] = true
	return nil
}

func (m *mockBlocklist) IsBlocked(_ context.Context, jti string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.blocked[jti], nil
}

// --- Helpers ---

func newJWT() *auth.JWTManager {
	return auth.NewJWTManager("test-secret-that-is-32-bytes-long", "velo")
}

func newBlocklist() *mockBlocklist {
	return &mockBlocklist{blocked: map[string]bool{}}
}

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// --- SignInWithApple ---

func TestSignInWithApple_Success(t *testing.T) {
	ctx := context.Background()
	user := domain.User{ID: uuid.New(), AppleSub: "sub_123"}
	apple := &mockApple{sub: "sub_123"}
	users := &mockUserRepo{upsertResult: user}
	tokens := &mockTokenRepo{}
	jwtMgr := newJWT()

	svc := service.NewAuthService(users, tokens, apple, jwtMgr, newBlocklist())
	accessToken, refreshToken, gotUser, err := svc.SignInWithApple(ctx, "apple-identity-token")

	require.NoError(t, err)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)
	require.Equal(t, user, gotUser)

	claims, err := jwtMgr.ParseAccessToken(accessToken)
	require.NoError(t, err)
	require.Equal(t, user.ID, claims.UserID)

	require.Equal(t, sha256hex(refreshToken), tokens.capturedHash)
}

func TestSignInWithApple_InvalidAppleToken(t *testing.T) {
	svc := service.NewAuthService(
		&mockUserRepo{},
		&mockTokenRepo{},
		&mockApple{err: errors.New("bad apple token")},
		newJWT(),
		newBlocklist(),
	)

	_, _, _, err := svc.SignInWithApple(context.Background(), "bad-token")
	require.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestSignInWithApple_EmptyAppleSub(t *testing.T) {
	svc := service.NewAuthService(
		&mockUserRepo{},
		&mockTokenRepo{},
		&mockApple{sub: ""}, // misbehaving validator returns empty sub with no error
		newJWT(),
		newBlocklist(),
	)

	_, _, _, err := svc.SignInWithApple(context.Background(), "token")
	require.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestSignInWithApple_UpsertError(t *testing.T) {
	svc := service.NewAuthService(
		&mockUserRepo{upsertErr: errors.New("db down")},
		&mockTokenRepo{},
		&mockApple{sub: "sub_123"},
		newJWT(),
		newBlocklist(),
	)

	_, _, _, err := svc.SignInWithApple(context.Background(), "valid-token")
	require.Error(t, err)
	require.False(t, errors.Is(err, domain.ErrUnauthorized))
}

// --- Refresh ---

func TestRefresh_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	jwtMgr := newJWT()

	tokens := &mockTokenRepo{
		getResult: domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}
	svc := service.NewAuthService(&mockUserRepo{}, tokens, &mockApple{}, jwtMgr, newBlocklist())

	accessToken, err := svc.Refresh(ctx, "some-refresh-token")
	require.NoError(t, err)
	require.NotEmpty(t, accessToken)

	claims, err := jwtMgr.ParseAccessToken(accessToken)
	require.NoError(t, err)
	require.Equal(t, userID, claims.UserID)
}

func TestRefresh_NotFound(t *testing.T) {
	tokens := &mockTokenRepo{getErr: domain.ErrNotFound}
	svc := service.NewAuthService(&mockUserRepo{}, tokens, &mockApple{}, newJWT(), newBlocklist())

	_, err := svc.Refresh(context.Background(), "nonexistent-token")
	require.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestRefresh_Expired(t *testing.T) {
	tokens := &mockTokenRepo{
		getResult: domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			ExpiresAt: time.Now().Add(-24 * time.Hour),
		},
	}
	svc := service.NewAuthService(&mockUserRepo{}, tokens, &mockApple{}, newJWT(), newBlocklist())

	_, err := svc.Refresh(context.Background(), "expired-token")
	require.True(t, errors.Is(err, domain.ErrUnauthorized))
}

// --- Logout ---

func TestLogout_RevokesRefreshAndBlocksJTI(t *testing.T) {
	ctx := context.Background()
	tokens := &mockTokenRepo{}
	bl := newBlocklist()
	jwtMgr := newJWT()

	userID := uuid.New()
	accessToken, err := jwtMgr.CreateAccessToken(userID)
	require.NoError(t, err)
	claims, err := jwtMgr.ParseAccessToken(accessToken)
	require.NoError(t, err)

	svc := service.NewAuthService(&mockUserRepo{}, tokens, &mockApple{}, jwtMgr, bl)
	err = svc.Logout(ctx, claims)

	require.NoError(t, err)
	require.True(t, tokens.deleteByUserIDCalled)
	require.True(t, bl.blocked[claims.JTI])
}

func TestLogout_DeleteByUserIDError(t *testing.T) {
	tokens := &mockTokenRepo{deleteByUserIDErr: errors.New("db down")}
	bl := newBlocklist()

	claims := auth.AccessTokenClaims{
		UserID:    uuid.New(),
		JTI:       uuid.NewString(),
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	svc := service.NewAuthService(&mockUserRepo{}, tokens, &mockApple{}, newJWT(), bl)
	err := svc.Logout(context.Background(), claims)

	require.Error(t, err)
	require.Empty(t, bl.blocked)
}

func TestLogout_BlocklistError(t *testing.T) {
	tokens := &mockTokenRepo{}
	bl := &mockBlocklist{blocked: map[string]bool{}, err: errors.New("redis down")}

	claims := auth.AccessTokenClaims{
		UserID:    uuid.New(),
		JTI:       uuid.NewString(),
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	svc := service.NewAuthService(&mockUserRepo{}, tokens, &mockApple{}, newJWT(), bl)
	err := svc.Logout(context.Background(), claims)

	require.Error(t, err)
	require.True(t, tokens.deleteByUserIDCalled)
}

func TestLogout_ExpiredAccessToken_SkipsBlock(t *testing.T) {
	tokens := &mockTokenRepo{}
	bl := newBlocklist()

	claims := auth.AccessTokenClaims{
		UserID:    uuid.New(),
		JTI:       uuid.NewString(),
		ExpiresAt: time.Now().Add(-5 * time.Minute),
	}

	svc := service.NewAuthService(&mockUserRepo{}, tokens, &mockApple{}, newJWT(), bl)
	err := svc.Logout(context.Background(), claims)

	require.NoError(t, err)
	require.True(t, tokens.deleteByUserIDCalled)
	require.Empty(t, bl.blocked)
}
