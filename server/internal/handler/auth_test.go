package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mockAuthService ---

type mockAuthService struct {
	signInAccessToken  string
	signInRefreshToken string
	signInUser         domain.User
	signInErr          error
	refreshAccessToken string
	refreshErr         error
	logoutErr          error
	logoutCalled       bool
}

func (m *mockAuthService) SignInWithApple(_ context.Context, _ string) (string, string, domain.User, error) {
	return m.signInAccessToken, m.signInRefreshToken, m.signInUser, m.signInErr
}

func (m *mockAuthService) Refresh(_ context.Context, _ string) (string, error) {
	return m.refreshAccessToken, m.refreshErr
}

func (m *mockAuthService) Logout(_ context.Context, _ auth.AccessTokenClaims) error {
	m.logoutCalled = true
	return m.logoutErr
}

var _ service.AuthService = (*mockAuthService)(nil)

// --- Logout handler tests ---

func TestLogout_Success(t *testing.T) {
	manager := auth.NewJWTManager("test-secret", "velo")
	svc := &mockAuthService{}
	h := NewAuthHandler(manager, svc)

	userID := uuid.New()
	token, err := manager.CreateAccessToken(userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, svc.logoutCalled)
}

func TestLogout_MissingToken(t *testing.T) {
	manager := auth.NewJWTManager("test-secret", "velo")
	svc := &mockAuthService{}
	h := NewAuthHandler(manager, svc)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, svc.logoutCalled)
}

func TestLogout_InvalidToken(t *testing.T) {
	manager := auth.NewJWTManager("test-secret", "velo")
	svc := &mockAuthService{}
	h := NewAuthHandler(manager, svc)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, svc.logoutCalled)
}

func TestLogout_ExpiredToken(t *testing.T) {
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	manager := auth.NewJWTManager("test-secret", "velo")
	manager.SetTimeFunc(func() time.Time { return now })

	userID := uuid.New()
	token, err := manager.CreateAccessToken(userID)
	require.NoError(t, err)

	manager.SetTimeFunc(func() time.Time { return now.Add(61 * time.Minute) })

	svc := &mockAuthService{}
	h := NewAuthHandler(manager, svc)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, svc.logoutCalled)
}

func TestLogout_ServiceError_Returns500(t *testing.T) {
	manager := auth.NewJWTManager("test-secret", "velo")
	svc := &mockAuthService{logoutErr: errors.New("redis down")}
	h := NewAuthHandler(manager, svc)

	userID := uuid.New()
	token, err := manager.CreateAccessToken(userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Apple handler tests ---

func TestApple_Success(t *testing.T) {
	userID := uuid.New()
	svc := &mockAuthService{
		signInAccessToken:  "access.tok",
		signInRefreshToken: "refresh.tok",
		signInUser:         domain.User{ID: userID, AppleSub: "sub_abc"},
	}
	h := NewAuthHandler(auth.NewJWTManager("secret", "velo"), svc)

	req := httptest.NewRequest(http.MethodPost, "/auth/apple",
		strings.NewReader(`{"identity_token":"valid-apple-tok"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Apple(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "access.tok", resp["access_token"])
	assert.Equal(t, "refresh.tok", resp["refresh_token"])
	assert.NotNil(t, resp["user"])
}

func TestApple_EmptyToken(t *testing.T) {
	h := NewAuthHandler(auth.NewJWTManager("secret", "velo"), &mockAuthService{})

	req := httptest.NewRequest(http.MethodPost, "/auth/apple",
		strings.NewReader(`{"identity_token":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Apple(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApple_InvalidJSON(t *testing.T) {
	h := NewAuthHandler(auth.NewJWTManager("secret", "velo"), &mockAuthService{})

	req := httptest.NewRequest(http.MethodPost, "/auth/apple",
		strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Apple(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApple_ServiceError(t *testing.T) {
	svc := &mockAuthService{signInErr: domain.ErrUnauthorized}
	h := NewAuthHandler(auth.NewJWTManager("secret", "velo"), svc)

	req := httptest.NewRequest(http.MethodPost, "/auth/apple",
		strings.NewReader(`{"identity_token":"bad-tok"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Apple(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Refresh handler tests ---

func TestRefresh_Success(t *testing.T) {
	svc := &mockAuthService{refreshAccessToken: "new.access.tok"}
	h := NewAuthHandler(auth.NewJWTManager("secret", "velo"), svc)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh",
		strings.NewReader(`{"refresh_token":"valid-refresh-tok"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "new.access.tok", resp["access_token"])
}

func TestRefresh_EmptyToken(t *testing.T) {
	h := NewAuthHandler(auth.NewJWTManager("secret", "velo"), &mockAuthService{})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh",
		strings.NewReader(`{"refresh_token":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRefresh_Expired(t *testing.T) {
	svc := &mockAuthService{refreshErr: domain.ErrUnauthorized}
	h := NewAuthHandler(auth.NewJWTManager("secret", "velo"), svc)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh",
		strings.NewReader(`{"refresh_token":"expired-tok"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Refresh(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
