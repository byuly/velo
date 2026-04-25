package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ ClipRepository = (*clipPg)(nil)

type clipPg struct {
	pool *pgxpool.Pool
}

func NewClipPg(pool *pgxpool.Pool) ClipRepository {
	return &clipPg{pool: pool}
}

const clipColumns = `id, session_id, user_id, slot_id, s3_key, recorded_at, arrived_at, recorded_at_clamped, duration_ms, created_at`

func scanClip(row pgx.Row) (domain.Clip, error) {
	var c domain.Clip
	err := row.Scan(&c.ID, &c.SessionID, &c.UserID, &c.SlotID, &c.S3Key,
		&c.RecordedAt, &c.ArrivedAt, &c.RecordedAtClamped, &c.DurationMs, &c.CreatedAt)
	return c, err
}

func (r *clipPg) Create(ctx context.Context, clip domain.Clip) (domain.Clip, error) {
	c, err := scanClip(r.pool.QueryRow(ctx, `
		INSERT INTO clips (session_id, user_id, slot_id, s3_key, recorded_at, arrived_at, recorded_at_clamped, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+clipColumns,
		clip.SessionID, clip.UserID, clip.SlotID, clip.S3Key,
		clip.RecordedAt, clip.ArrivedAt, clip.RecordedAtClamped, clip.DurationMs))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Clip{}, domain.ErrDuplicateClip
		}
		return domain.Clip{}, fmt.Errorf("create clip: %w", err)
	}
	return c, nil
}

func (r *clipPg) GetByID(ctx context.Context, id uuid.UUID) (domain.Clip, error) {
	c, err := scanClip(r.pool.QueryRow(ctx, `SELECT `+clipColumns+` FROM clips WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Clip{}, domain.ErrNotFound
		}
		return domain.Clip{}, fmt.Errorf("get clip by id: %w", err)
	}
	return c, nil
}

func (r *clipPg) GetBySessionID(ctx context.Context, sessionID uuid.UUID) ([]domain.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+clipColumns+` FROM clips WHERE session_id = $1 ORDER BY recorded_at ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get clips by session: %w", err)
	}
	return collectClips(rows, "get clips by session")
}

func (r *clipPg) GetBySessionAndUser(ctx context.Context, sessionID, userID uuid.UUID) ([]domain.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+clipColumns+` FROM clips WHERE session_id = $1 AND user_id = $2 ORDER BY recorded_at ASC`,
		sessionID, userID)
	if err != nil {
		return nil, fmt.Errorf("get clips by session and user: %w", err)
	}
	return collectClips(rows, "get clips by session and user")
}

func (r *clipPg) GetTotalDurationForSlot(ctx context.Context, slotID uuid.UUID) (int, error) {
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(duration_ms), 0)::int FROM clips WHERE slot_id = $1`, slotID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get total duration for slot: %w", err)
	}
	return total, nil
}

func collectClips(rows pgx.Rows, errCtx string) ([]domain.Clip, error) {
	defer rows.Close()
	clips := []domain.Clip{}
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", errCtx, err)
		}
		clips = append(clips, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", errCtx, err)
	}
	return clips, nil
}
