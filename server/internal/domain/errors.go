package domain

import "fmt"

// AppError represents a structured application error with an HTTP status code.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *AppError) Error() string { return e.Message }

func NewAppError(code, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, Status: status}
}

func ValidationError(message string) *AppError {
	return &AppError{Code: "INVALID_INPUT", Message: message, Status: 400}
}

func ValidationErrorf(format string, args ...any) *AppError {
	return ValidationError(fmt.Sprintf(format, args...))
}

// Predefined application errors.
var (
	ErrNotFound         = NewAppError("NOT_FOUND", "resource not found", 404)
	ErrUnauthorized     = NewAppError("UNAUTHORIZED", "unauthorized", 401)
	ErrForbidden        = NewAppError("FORBIDDEN", "forbidden", 403)
	ErrSessionFull      = NewAppError("SESSION_FULL", "session is full", 409)
	ErrAlreadyInSession = NewAppError("ALREADY_IN_SESSION", "already in a session", 409)
	ErrSessionNotActive = NewAppError("SESSION_NOT_ACTIVE", "session is not active", 409)
	ErrInviteExpired    = NewAppError("INVITE_EXPIRED", "invite has expired", 410)
	ErrDuplicateClip    = NewAppError("DUPLICATE_CLIP", "duplicate clip", 409)
	ErrInvalidInput     = NewAppError("INVALID_INPUT", "invalid input", 400)
)
