package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/handler"
)

func Auth(manager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				slog.Warn("missing or malformed authorization header",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				handler.Error(w, domain.ErrUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := manager.ParseAccessToken(token)
			if err != nil {
				slog.Warn("invalid access token",
					slog.String("error", err.Error()),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				handler.Error(w, domain.ErrUnauthorized)
				return
			}

			ctx := handler.SetUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
