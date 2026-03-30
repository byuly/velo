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

func TestAppleValidator_WrongSigningKey(t *testing.T) {
	_, server, keyID := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	// Sign with a completely different RSA key — JWKS has the real key, not this one.
	attackerKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, attackerKey, keyID, appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	_, err = validator.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestAppleValidator_UnknownKID(t *testing.T) {
	privateKey, server, _ := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, "kid-not-in-jwks", appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	_, err := validator.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestAppleValidator_MissingKID(t *testing.T) {
	privateKey, server, _ := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	// Build a token without setting kid in the header.
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   "apple-user-123",
		Audience:  jwt.ClaimStrings{"com.example.velo"},
		Issuer:    appleIssuer,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := tok.SignedString(privateKey)
	require.NoError(t, err)

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	_, err = validator.Validate(context.Background(), signed)
	require.Error(t, err)
}

func TestAppleValidator_EmptySub(t *testing.T) {
	privateKey, server, keyID := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, keyID, appleTokenOptions{
		sub: "",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	_, err := validator.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestAppleValidator_MalformedToken(t *testing.T) {
	_, server, _ := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	_, err := validator.Validate(context.Background(), "not.a.jwt")
	require.Error(t, err)
}

func TestAppleValidator_WrongSigningMethod(t *testing.T) {
	_, server, keyID := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "apple-user-123",
		Audience:  jwt.ClaimStrings{"com.example.velo"},
		Issuer:    appleIssuer,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tok.Header["kid"] = keyID
	signed, err := tok.SignedString([]byte("hmac-secret"))
	require.NoError(t, err)

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	_, err = validator.Validate(context.Background(), signed)
	require.Error(t, err)
}

func TestAppleValidator_InvalidJWKSJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json{{{"))
	}))
	defer server.Close()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, "test-key-id", appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	_, err = validator.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestAppleValidator_KeyRotation(t *testing.T) {
	key1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	key2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwks := appleJWKS{
		Keys: []appleJWK{
			{
				Kid: "key-1", Kty: "RSA", Alg: "RS256", Use: "sig",
				N: base64.RawURLEncoding.EncodeToString(key1.PublicKey.N.Bytes()),
				E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key1.PublicKey.E)).Bytes()),
			},
			{
				Kid: "key-2", Kty: "RSA", Alg: "RS256", Use: "sig",
				N: base64.RawURLEncoding.EncodeToString(key2.PublicKey.N.Bytes()),
				E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key2.PublicKey.E)).Bytes()),
			},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(jwks))
	}))
	defer server.Close()

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, key2, "key-2", appleTokenOptions{
		sub: "apple-user-999",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	appleSub, err := validator.Validate(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "apple-user-999", appleSub)
}

func TestAppleValidator_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The request should never reach here.
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	token := mustCreateAppleIdentityToken(t, privateKey, "test-key-id", appleTokenOptions{
		sub: "apple-user-123",
		aud: "com.example.velo",
		iss: appleIssuer,
		exp: time.Now().Add(time.Hour),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = validator.Validate(ctx, token)
	require.Error(t, err)
}

func TestAppleValidator_MissingExpiry(t *testing.T) {
	privateKey, server, keyID := newAppleTestServer(t, http.StatusOK)
	defer server.Close()

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:  "apple-user-123",
		Audience: jwt.ClaimStrings{"com.example.velo"},
		Issuer:   appleIssuer,
		IssuedAt: jwt.NewNumericDate(time.Now()),
		// No ExpiresAt — WithExpirationRequired should reject this.
	})
	tok.Header["kid"] = keyID
	signed, err := tok.SignedString(privateKey)
	require.NoError(t, err)

	validator := newTestAppleValidator(server.URL, "com.example.velo")
	_, err = validator.Validate(context.Background(), signed)
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
