package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/service"
)

type AuthHandler struct {
	jwt       *auth.JWTManager
	blocklist auth.TokenBlocklist
	svc       service.AuthService
}

func NewAuthHandler(jwt *auth.JWTManager, blocklist auth.TokenBlocklist, svc service.AuthService) *AuthHandler {
	return &AuthHandler{jwt: jwt, blocklist: blocklist, svc: svc}
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
		Error(w, fmt.Errorf("revoke token: %w", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Apple(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IdentityToken string `json:"identity_token"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, err)
		return
	}
	if body.IdentityToken == "" {
		Error(w, domain.ValidationError("identity_token is required"))
		return
	}

	accessToken, refreshToken, user, err := h.svc.SignInWithApple(r.Context(), body.IdentityToken)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          user,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, err)
		return
	}
	if body.RefreshToken == "" {
		Error(w, domain.ValidationError("refresh_token is required"))
		return
	}

	accessToken, err := h.svc.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
	})
}
