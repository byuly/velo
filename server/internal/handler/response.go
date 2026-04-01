package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
)

// --- Context helpers ---

type ctxKey int

const userIDKey ctxKey = iota

func SetUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

// --- Response ---

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", slog.String("error", err.Error()))
	}
}

type errorEnvelope struct {
	Error *domain.AppError `json:"error"`
}

func Error(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		JSON(w, appErr.Status, errorEnvelope{Error: appErr})
		return
	}

	slog.Error("internal error", slog.String("error", err.Error()))
	internal := domain.NewAppError("INTERNAL_ERROR", "internal error", http.StatusInternalServerError)
	JSON(w, internal.Status, errorEnvelope{Error: internal})
}

// --- Request ---

const maxBodySize = 1 << 20 // 1 MB

func Decode(r *http.Request, v any) error {
	body := io.LimitReader(r.Body, maxBodySize)
	decoder := json.NewDecoder(body)

	if err := decoder.Decode(v); err != nil {
		if err == io.EOF {
			return domain.ValidationError("request body must not be empty")
		}
		return domain.ValidationError("invalid JSON: " + err.Error())
	}

	return nil
}
