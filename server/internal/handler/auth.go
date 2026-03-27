package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/byuly/velo/server/internal/domain"
)

type AuthHandler struct {
	jwt       *auth.JWTManager
	blocklist auth.TokenBlocklist
}

func NewAuthHandler(jwt *auth.JWTManager, blocklist auth.TokenBlocklist) *AuthHandler {
	return &AuthHandler{jwt: jwt, blocklist: blocklist}
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		Error(w, domain.ErrUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := h.jwt.ParseAccessToken(token)
	if err != nil {
		Error(w, domain.ErrUnauthorized)
		return
	}

	remainingTTL := time.Until(claims.ExpiresAt)
	if remainingTTL <= 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.blocklist.Block(r.Context(), claims.JTI, remainingTTL); err != nil {
		slog.Error("failed to revoke token",
			slog.String("error", err.Error()),
			slog.String("jti", claims.JTI),
		)
	}

	w.WriteHeader(http.StatusNoContent)
}
