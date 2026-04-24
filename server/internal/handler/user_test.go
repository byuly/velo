package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mockUserService ---

type mockUserService struct {
	getMeUser domain.User
	getMeErr  error

	updateMeIn   domain.User
	updateMeUser domain.User
	updateMeErr  error

	deleteMeCalled bool
	deleteMeErr    error
}

func (m *mockUserService) GetMe(_ context.Context, _ uuid.UUID) (domain.User, error) {
	return m.getMeUser, m.getMeErr
}

func (m *mockUserService) UpdateMe(_ context.Context, _ uuid.UUID, update domain.User) (domain.User, error) {
	m.updateMeIn = update
	return m.updateMeUser, m.updateMeErr
}

func (m *mockUserService) DeleteMe(_ context.Context, _ uuid.UUID) error {
	m.deleteMeCalled = true
	return m.deleteMeErr
}

var _ service.UserService = (*mockUserService)(nil)

func authedReq(method, url, body string, userID uuid.UUID) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, url, nil)
	} else {
		r = httptest.NewRequest(method, url, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	return r.WithContext(SetUserID(r.Context(), userID))
}

// --- GetMe ---

func TestGetMe_Success(t *testing.T) {
	userID := uuid.New()
	svc := &mockUserService{getMeUser: domain.User{ID: userID}}
	h := NewUserHandler(svc)

	w := httptest.NewRecorder()
	h.GetMe(w, authedReq(http.MethodGet, "/users/me", "", userID))

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotNil(t, resp["user"])
}

func TestGetMe_NoUserInContext(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	w := httptest.NewRecorder()
	h.GetMe(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetMe_ServiceError(t *testing.T) {
	svc := &mockUserService{getMeErr: domain.ErrNotFound}
	h := NewUserHandler(svc)

	w := httptest.NewRecorder()
	h.GetMe(w, authedReq(http.MethodGet, "/users/me", "", uuid.New()))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- UpdateMe ---

func TestUpdateMe_Success(t *testing.T) {
	userID := uuid.New()
	newName := "velo"
	svc := &mockUserService{updateMeUser: domain.User{ID: userID, DisplayName: &newName}}
	h := NewUserHandler(svc)

	w := httptest.NewRecorder()
	h.UpdateMe(w, authedReq(http.MethodPatch, "/users/me", `{"display_name":"velo"}`, userID))

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.updateMeIn.DisplayName)
	assert.Equal(t, "velo", *svc.updateMeIn.DisplayName)
}

func TestUpdateMe_NoUserInContext(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	req := httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"display_name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.UpdateMe(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateMe_TooLong(t *testing.T) {
	svc := &mockUserService{updateMeErr: domain.ValidationErrorf("display_name must be at most %d characters", domain.MaxDisplayNameLength)}
	h := NewUserHandler(svc)

	tooLong := strings.Repeat("a", domain.MaxDisplayNameLength+1)
	body := `{"display_name":"` + tooLong + `"}`
	w := httptest.NewRecorder()
	h.UpdateMe(w, authedReq(http.MethodPatch, "/users/me", body, uuid.New()))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateMe_InvalidJSON(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	w := httptest.NewRecorder()
	h.UpdateMe(w, authedReq(http.MethodPatch, "/users/me", `not-json`, uuid.New()))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- DeleteMe ---

func TestDeleteMe_Success(t *testing.T) {
	svc := &mockUserService{}
	h := NewUserHandler(svc)

	w := httptest.NewRecorder()
	h.DeleteMe(w, authedReq(http.MethodDelete, "/users/me", "", uuid.New()))

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, svc.deleteMeCalled)
}

func TestDeleteMe_NoUserInContext(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	req := httptest.NewRequest(http.MethodDelete, "/users/me", nil)
	w := httptest.NewRecorder()
	h.DeleteMe(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteMe_ServiceError(t *testing.T) {
	svc := &mockUserService{deleteMeErr: errors.New("db down")}
	h := NewUserHandler(svc)

	w := httptest.NewRecorder()
	h.DeleteMe(w, authedReq(http.MethodDelete, "/users/me", "", uuid.New()))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
