package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_ImplementsError(t *testing.T) {
	var err error = NewAppError("TEST", "test message", 400)
	assert.Equal(t, "test message", err.Error())
}

func TestPredefinedErrors_UniqueCodes(t *testing.T) {
	errors := []*AppError{
		ErrNotFound, ErrUnauthorized, ErrForbidden,
		ErrSessionFull, ErrAlreadyInSession, ErrSessionNotActive,
		ErrInviteExpired, ErrDuplicateClip, ErrInvalidInput,
	}

	seen := make(map[string]bool)
	for _, e := range errors {
		assert.False(t, seen[e.Code], "duplicate code: %s", e.Code)
		seen[e.Code] = true
	}
}

func TestPredefinedErrors_StatusCodes(t *testing.T) {
	tests := []struct {
		err    *AppError
		status int
	}{
		{ErrNotFound, 404},
		{ErrUnauthorized, 401},
		{ErrForbidden, 403},
		{ErrSessionFull, 409},
		{ErrAlreadyInSession, 409},
		{ErrSessionNotActive, 409},
		{ErrInviteExpired, 410},
		{ErrDuplicateClip, 409},
		{ErrInvalidInput, 400},
	}

	for _, tc := range tests {
		t.Run(tc.err.Code, func(t *testing.T) {
			assert.Equal(t, tc.status, tc.err.Status)
		})
	}
}

func TestValidationError_CustomMessage(t *testing.T) {
	err := ValidationError("name too long")
	assert.Equal(t, "INVALID_INPUT", err.Code)
	assert.Equal(t, "name too long", err.Message)
	assert.Equal(t, 400, err.Status)
}

func TestValidationErrorf(t *testing.T) {
	err := ValidationErrorf("field %q must be at most %d chars", "name", 40)
	assert.Equal(t, "INVALID_INPUT", err.Code)
	assert.Contains(t, err.Message, "name")
	assert.Contains(t, err.Message, "40")
}
