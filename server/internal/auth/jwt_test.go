package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_CreateAndParseAccessToken(t *testing.T) {
	manager := NewJWTManager("test-secret", "velo")
	userID := uuid.New()

	token, err := manager.CreateAccessToken(userID)
	require.NoError(t, err)

	got, err := manager.ParseAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestJWTManager_ParseAccessToken_ExpiredToken(t *testing.T) {
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	manager := NewJWTManager("test-secret", "velo")
	manager.now = func() time.Time { return now }

	userID := uuid.New()
	token, err := manager.CreateAccessToken(userID)
	require.NoError(t, err)

	manager.now = func() time.Time { return now.Add(accessTokenTTL + time.Second) }

	_, err = manager.ParseAccessToken(token)
	require.Error(t, err)
}

func TestJWTManager_ParseAccessToken_WrongSigningMethod(t *testing.T) {
	claims := accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "velo",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	manager := NewJWTManager("test-secret", "velo")

	_, err = manager.ParseAccessToken(signed)
	require.Error(t, err)
}

func TestJWTManager_ParseAccessToken_MalformedToken(t *testing.T) {
	manager := NewJWTManager("test-secret", "velo")

	_, err := manager.ParseAccessToken("not-a-jwt")
	require.Error(t, err)
}
