package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestLogout_Success(t *testing.T) {
	manager := auth.NewJWTManager("test-secret", "velo")
	bl := &mockBlocklist{blocked: map[string]bool{}}
	h := NewAuthHandler(manager, bl)

	userID := uuid.New()
	token, err := manager.CreateAccessToken(userID)
	require.NoError(t, err)

	claims, err := manager.ParseAccessToken(token)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, bl.blocked[claims.JTI])
}

func TestLogout_MissingToken(t *testing.T) {
	manager := auth.NewJWTManager("test-secret", "velo")
	bl := &mockBlocklist{blocked: map[string]bool{}}
	h := NewAuthHandler(manager, bl)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogout_InvalidToken(t *testing.T) {
	manager := auth.NewJWTManager("test-secret", "velo")
	bl := &mockBlocklist{blocked: map[string]bool{}}
	h := NewAuthHandler(manager, bl)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogout_ExpiredToken(t *testing.T) {
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	manager := auth.NewJWTManager("test-secret", "velo")
	manager.SetTimeFunc(func() time.Time { return now })

	userID := uuid.New()
	token, err := manager.CreateAccessToken(userID)
	require.NoError(t, err)

	// Advance time past expiry — ParseAccessToken rejects expired tokens
	manager.SetTimeFunc(func() time.Time { return now.Add(61 * time.Minute) })

	bl := &mockBlocklist{blocked: map[string]bool{}}
	h := NewAuthHandler(manager, bl)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, bl.blocked)
}

func TestLogout_BlocklistError_BestEffort(t *testing.T) {
	manager := auth.NewJWTManager("test-secret", "velo")
	bl := &mockBlocklist{blocked: map[string]bool{}, err: errors.New("redis down")}
	h := NewAuthHandler(manager, bl)

	userID := uuid.New()
	token, err := manager.CreateAccessToken(userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
