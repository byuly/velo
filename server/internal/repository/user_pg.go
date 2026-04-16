package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface check.
var _ UserRepository = (*userPg)(nil)

type userPg struct {
	pool *pgxpool.Pool
}

// NewUserPg returns a UserRepository backed by pgxpool.
func NewUserPg(pool *pgxpool.Pool) UserRepository {
	return &userPg{pool: pool}
}

// scanUser scans a row into domain.User using the canonical column order:
// id, apple_sub, display_name, avatar_url, apns_token, created_at, updated_at
func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.AppleSub, &u.DisplayName, &u.AvatarURL, &u.APNsToken, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *userPg) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT id, apple_sub, display_name, avatar_url, apns_token, created_at, updated_at
		FROM users WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (r *userPg) GetByAppleSub(ctx context.Context, sub string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT id, apple_sub, display_name, avatar_url, apns_token, created_at, updated_at
		FROM users WHERE apple_sub = $1`, sub))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by apple_sub: %w", err)
	}
	return u, nil
}

func (r *userPg) UpsertByAppleSub(ctx context.Context, sub string) (domain.User, error) {
	// DO UPDATE SET apple_sub = EXCLUDED.apple_sub is a no-op trick to satisfy
	// RETURNING on conflict. updated_at is intentionally not touched — no data changed.
	u, err := scanUser(r.pool.QueryRow(ctx, `
		INSERT INTO users (apple_sub) VALUES ($1)
		ON CONFLICT (apple_sub) DO UPDATE SET apple_sub = EXCLUDED.apple_sub
		RETURNING id, apple_sub, display_name, avatar_url, apns_token, created_at, updated_at`, sub))
	if err != nil {
		return domain.User{}, fmt.Errorf("upsert user by apple_sub: %w", err)
	}
	return u, nil
}

func (r *userPg) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *userPg) UpdateDisplayName(ctx context.Context, id uuid.UUID, name string) error {
	return r.updateNullableText(ctx, id, "display_name", name, "update display name")
}

func (r *userPg) UpdateAvatarURL(ctx context.Context, id uuid.UUID, url string) error {
	return r.updateNullableText(ctx, id, "avatar_url", url, "update avatar url")
}

func (r *userPg) UpdateAPNsToken(ctx context.Context, id uuid.UUID, token string) error {
	return r.updateNullableText(ctx, id, "apns_token", token, "update apns token")
}

// updateNullableText sets a single nullable TEXT column. Empty value stores
// NULL; column name is a constant at every call site, never user input.
func (r *userPg) updateNullableText(ctx context.Context, id uuid.UUID, column, value, errCtx string) error {
	var v *string
	if value != "" {
		v = &value
	}
	query := fmt.Sprintf(`UPDATE users SET %s = $2, updated_at = now() WHERE id = $1`, column)
	tag, err := r.pool.Exec(ctx, query, id, v)
	if err != nil {
		return fmt.Errorf("%s: %w", errCtx, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
