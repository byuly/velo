package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	appleIssuer  = "https://appleid.apple.com"
	appleJWKSURL = "https://appleid.apple.com/auth/keys"
)

type AppleValidator interface {
	Validate(ctx context.Context, identityToken string) (appleSub string, err error)
}

type appleTokenValidator struct {
	appID      string
	jwksURL    string
	httpClient *http.Client
	now        func() time.Time
}

type appleIdentityClaims struct {
	jwt.RegisteredClaims
}

type appleJWKS struct {
	Keys []appleJWK `json:"keys"`
}

type appleJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func NewAppleValidator(appID string) AppleValidator {
	return &appleTokenValidator{
		appID:   appID,
		jwksURL: appleJWKSURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		now: time.Now,
	}
}

func (v *appleTokenValidator) Validate(ctx context.Context, identityToken string) (string, error) {
	keyID, err := keyIDFromToken(identityToken)
	if err != nil {
		return "", err
	}

	keySet, err := v.fetchJWKS(ctx)
	if err != nil {
		return "", err
	}

	publicKey, err := keySet.publicKey(keyID)
	if err != nil {
		return "", err
	}

	parser := jwt.NewParser(
		jwt.WithTimeFunc(v.now),
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(v.appID),
		jwt.WithExpirationRequired(),
	)

	token, err := parser.ParseWithClaims(identityToken, &appleIdentityClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodRS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return publicKey, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse apple identity token: %w", err)
	}

	claims, ok := token.Claims.(*appleIdentityClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid apple identity token claims")
	}
	if claims.Subject == "" {
		return "", errors.New("apple identity token missing sub")
	}

	return claims.Subject, nil
}

func keyIDFromToken(identityToken string) (string, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))

	token, _, err := parser.ParseUnverified(identityToken, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("parse token header: %w", err)
	}

	keyID, ok := token.Header["kid"].(string)
	if !ok || keyID == "" {
		return "", errors.New("apple identity token missing kid")
	}

	return keyID, nil
}

func (v *appleTokenValidator) fetchJWKS(ctx context.Context) (*appleJWKS, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch jwks: unexpected status %d", resp.StatusCode)
	}

	var keySet appleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&keySet); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}

	return &keySet, nil
}

func (s *appleJWKS) publicKey(keyID string) (*rsa.PublicKey, error) {
	for _, key := range s.Keys {
		if key.Kid != keyID {
			continue
		}
		return key.rsaPublicKey()
	}

	return nil, fmt.Errorf("apple jwks key %q not found", keyID)
}

func (k *appleJWK) rsaPublicKey() (*rsa.PublicKey, error) {
	if k.Kty != "RSA" {
		return nil, fmt.Errorf("unexpected key type: %s", k.Kty)
	}
	if k.N == "" || k.E == "" {
		return nil, errors.New("apple jwk missing modulus or exponent")
	}

	modulusBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}

	exponentBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	modulus := new(big.Int).SetBytes(modulusBytes)
	exponent := new(big.Int).SetBytes(exponentBytes)
	if exponent.Sign() <= 0 {
		return nil, errors.New("invalid rsa exponent")
	}

	return &rsa.PublicKey{
		N: modulus,
		E: int(exponent.Int64()),
	}, nil
}
