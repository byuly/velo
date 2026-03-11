package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- JSON tests ---

func TestJSON_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusOK, map[string]string{"key": "value"})
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestJSON_StatusCode(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"200", http.StatusOK},
		{"201", http.StatusCreated},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			JSON(w, tc.status, map[string]string{})
			assert.Equal(t, tc.status, w.Code)
		})
	}
}

func TestJSON_BodyEncoding(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusOK, map[string]string{"hello": "world"})

	var body map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "world", body["hello"])
}

// --- Error tests ---

func TestError_AppError(t *testing.T) {
	tests := []struct {
		name   string
		err    *domain.AppError
		status int
		code   string
	}{
		{"not found", domain.ErrNotFound, 404, "NOT_FOUND"},
		{"session full", domain.ErrSessionFull, 409, "SESSION_FULL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			Error(w, tc.err)

			assert.Equal(t, tc.status, w.Code)

			var body map[string]map[string]string
			err := json.NewDecoder(w.Body).Decode(&body)
			require.NoError(t, err)
			assert.Equal(t, tc.code, body["error"]["code"])
		})
	}
}

func TestError_ValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, domain.ValidationError("name too long"))

	assert.Equal(t, 400, w.Code)

	var body map[string]map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_INPUT", body["error"]["code"])
	assert.Equal(t, "name too long", body["error"]["message"])
}

func TestError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, fmt.Errorf("something broke"))

	assert.Equal(t, 500, w.Code)

	var body map[string]map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", body["error"]["code"])
	assert.Equal(t, "internal error", body["error"]["message"])
}

func TestError_WrappedAppError(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", domain.ErrForbidden)
	w := httptest.NewRecorder()
	Error(w, wrapped)

	assert.Equal(t, 403, w.Code)

	var body map[string]map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "FORBIDDEN", body["error"]["code"])
}

// --- Decode tests ---

func TestDecode_ValidJSON(t *testing.T) {
	type req struct {
		Name string `json:"name"`
	}

	body := strings.NewReader(`{"name": "test"}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var v req
	err := Decode(r, &v)
	require.NoError(t, err)
	assert.Equal(t, "test", v.Name)
}

func TestDecode_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{invalid}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var v map[string]string
	err := Decode(r, &v)
	require.Error(t, err)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "INVALID_INPUT", appErr.Code)
	assert.Contains(t, appErr.Message, "invalid JSON")
}

func TestDecode_EmptyBody(t *testing.T) {
	body := strings.NewReader("")
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var v map[string]string
	err := Decode(r, &v)
	require.Error(t, err)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "empty")
}

func TestDecode_ExtraFieldsIgnored(t *testing.T) {
	type req struct {
		Name string `json:"name"`
	}

	body := strings.NewReader(`{"name": "test", "unknown_field": 42}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var v req
	err := Decode(r, &v)
	require.NoError(t, err)
	assert.Equal(t, "test", v.Name)
}

// --- UserID context tests ---

func TestUserID_RoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := SetUserID(context.Background(), id)
	got := UserID(ctx)
	assert.Equal(t, id, got)
}

func TestUserID_PanicsWhenMissing(t *testing.T) {
	assert.Panics(t, func() {
		UserID(context.Background())
	})
}
