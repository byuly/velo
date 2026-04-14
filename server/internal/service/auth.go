package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/repository"
)

// AuthService handles Sign In with Apple and token refresh.
type AuthService interface {
	SignInWithApple(ctx context.Context, identityToken string) (accessToken, refreshToken string, user domain.User, err error)
	Refresh(ctx context.Context, refreshToken string) (accessToken string, err error)
}

// Compile-time interface check.
var _ AuthService = (*authService)(nil)

type authService struct {
	users  repository.UserRepository
	tokens repository.TokenRepository
	apple  auth.AppleValidator
	jwt    *auth.JWTManager
}

func NewAuthService(
	users repository.UserRepository,
	tokens repository.TokenRepository,
	apple auth.AppleValidator,
	jwt *auth.JWTManager,
) AuthService {
	return &authService{users: users, tokens: tokens, apple: apple, jwt: jwt}
}

const refreshTokenTTL = 90 * 24 * time.Hour

func (s *authService) SignInWithApple(ctx context.Context, identityToken string) (string, string, domain.User, error) {
	appleSub, err := s.apple.Validate(ctx, identityToken)
	if err != nil {
		return "", "", domain.User{}, domain.ErrUnauthorized
	}

	user, err := s.users.UpsertByAppleSub(ctx, appleSub)
	if err != nil {
		return "", "", domain.User{}, fmt.Errorf("sign in with apple: %w", err)
	}

	accessToken, err := s.jwt.CreateAccessToken(user.ID)
	if err != nil {
		return "", "", domain.User{}, fmt.Errorf("create access token: %w", err)
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return "", "", domain.User{}, fmt.Errorf("generate refresh token: %w", err)
	}

	if _, err := s.tokens.Create(ctx, user.ID, hashToken(refreshToken), time.Now().Add(refreshTokenTTL)); err != nil {
		return "", "", domain.User{}, fmt.Errorf("store refresh token: %w", err)
	}

	return accessToken, refreshToken, user, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (string, error) {
	stored, err := s.tokens.GetByHash(ctx, hashToken(refreshToken))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", domain.ErrUnauthorized
		}
		return "", fmt.Errorf("refresh token lookup: %w", err)
	}

	if stored.ExpiresAt.Before(time.Now()) {
		return "", domain.ErrUnauthorized
	}

	accessToken, err := s.jwt.CreateAccessToken(stored.UserID)
	if err != nil {
		return "", fmt.Errorf("create access token: %w", err)
	}

	return accessToken, nil
}

// generateRefreshToken returns a cryptographically random 256-bit token
// encoded as a base64url string (~43 chars).
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the hex-encoded SHA-256 hash of the token.
// This is what gets stored in the database — the raw token is never persisted.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
