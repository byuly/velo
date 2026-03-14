package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppleValidator_ValidToken(t *testing.T) {
	privateKey, server, keyID := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, keyID, appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	appleSub, err := validator.Validate(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "apple-user-123", appleSub)
}

func TestAppleValidator_ExpiredToken(t *testing.T) {
	privateKey, server, keyID := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, keyID, appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(-time.Hour),
	})

	_, err := validator.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestAppleValidator_WrongAudience(t *testing.T) {
	privateKey, server, keyID := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, keyID, appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.other.app",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	_, err := validator.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestAppleValidator_WrongIssuer(t *testing.T) {
	privateKey, server, keyID := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, keyID, appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.example.velo",
		iss: "https://example.com",
		exp: time.Now().Add(time.Hour),
	})

	_, err := validator.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestAppleValidator_JWKSFetchFailure(t *testing.T) {
	privateKey, server, keyID := newAppleTestServer(t, http.StatusInternalServerError)
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, keyID, appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	_, err := validator.Validate(context.Background(), token)
	require.Error(t, err)
}

type appleTokenOptions struct {
	sub string
	aud string
	iss string
	exp time.Time
}

func newTestAppleValidator(jwksURL, appID string) *appleTokenValidator {
	return &appleTokenValidator{
		appID:      appID,
		jwksURL:    jwksURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		now:        time.Now,
	}
}

func newAppleTestServer(t *testing.T, status int) (*rsa.PrivateKey, *httptest.Server, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyID := "test-key-id"
	jwks := appleJWKS{
		Keys: []appleJWK{
			{
				Kid: keyID,
				Kty: "RSA",
				Alg: "RS256",
				Use: "sig",
				N:   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if status != http.StatusOK {
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(jwks))
	}))

	return privateKey, server, keyID
}

func mustCreateAppleIdentityToken(t *testing.T, privateKey *rsa.PrivateKey, keyID string, opts appleTokenOptions) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   opts.sub,
		Audience:  jwt.ClaimStrings{opts.aud},
		Issuer:    opts.iss,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(opts.exp),
	})
	token.Header["kid"] = keyID

	signed, err := token.SignedString(privateKey)
	require.NoError(t, err)

	return signed
}
