package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TODO: pair with a refresh token flow for shorter-lived access tokens.
const accessTokenTTL = 60 * time.Minute

type JWTManager struct {
	secret []byte
	issuer string
	now    func() time.Time
}

type accessTokenClaims struct {
	jwt.RegisteredClaims
}

func NewJWTManager(secret, issuer string) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		issuer: issuer,
		now:    time.Now,
	}
}

// AccessTokenClaims holds the parsed fields from a validated access token.
type AccessTokenClaims struct {
	UserID    uuid.UUID
	JTI       string
	ExpiresAt time.Time
}

func (m *JWTManager) CreateAccessToken(userID uuid.UUID) (string, error) {
	claims := accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   userID.String(),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(m.now()),
			NotBefore: jwt.NewNumericDate(m.now()),
			ExpiresAt: jwt.NewNumericDate(m.now().Add(accessTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}

	return signed, nil
}

func (m *JWTManager) ParseAccessToken(tokenString string) (AccessTokenClaims, error) {
	parser := jwt.NewParser(
		jwt.WithTimeFunc(m.now),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)

	token, err := parser.ParseWithClaims(tokenString, &accessTokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.secret, nil
	})
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("parse access token: %w", err)
	}

	claims, ok := token.Claims.(*accessTokenClaims)
	if !ok || !token.Valid {
		return AccessTokenClaims{}, errors.New("invalid access token claims")
	}

	if claims.ID == "" {
		return AccessTokenClaims{}, errors.New("missing jti claim")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("parse subject uuid: %w", err)
	}

	return AccessTokenClaims{
		UserID:    userID,
		JTI:       claims.ID,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

// SetTimeFunc overrides the time function used for token creation and parsing.
// This is intended for testing.
func (m *JWTManager) SetTimeFunc(fn func() time.Time) {
	m.now = fn
}
