package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface check.
var _ TokenRepository = (*tokenPg)(nil)

type tokenPg struct {
	pool *pgxpool.Pool
}

// NewTokenPg returns a TokenRepository backed by pgxpool.
func NewTokenPg(pool *pgxpool.Pool) TokenRepository {
	return &tokenPg{pool: pool}
}

// scanToken scans a row into domain.RefreshToken using the canonical column order:
// id, user_id, token_hash, expires_at, created_at
func scanToken(row pgx.Row) (domain.RefreshToken, error) {
	var rt domain.RefreshToken
	err := row.Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.CreatedAt)
	return rt, err
}

func (r *tokenPg) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (domain.RefreshToken, error) {
	rt, err := scanToken(r.pool.QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, expires_at, created_at`,
		userID, tokenHash, expiresAt))
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("create refresh token: %w", err)
	}
	return rt, nil
}

func (r *tokenPg) GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	// Returns the token regardless of expiry — the service layer decides
	// whether an expired token is an error, keeping ErrNotFound distinct from expiry.
	rt, err := scanToken(r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM refresh_tokens WHERE token_hash = $1`, hash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefreshToken{}, domain.ErrNotFound
		}
		return domain.RefreshToken{}, fmt.Errorf("get refresh token by hash: %w", err)
	}
	return rt, nil
}

func (r *tokenPg) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *tokenPg) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	// No ErrNotFound when zero rows deleted — zero tokens is a valid state
	// (e.g., revoking sessions for a user who has already logged out everywhere).
	_, err := r.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete refresh tokens by user: %w", err)
	}
	return nil
}
