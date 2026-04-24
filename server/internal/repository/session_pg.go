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

var _ SessionRepository = (*sessionPg)(nil)

type sessionPg struct {
	pool *pgxpool.Pool
}

func NewSessionPg(pool *pgxpool.Pool) SessionRepository {
	return &sessionPg{pool: pool}
}

// sessionColumns is the canonical column list for session row scans.
const sessionColumns = `id, creator_id, name, mode, section_count, max_section_duration_s,
	deadline, invite_token, status, reel_url, retry_count,
	reminder_2h_sent, reminder_30m_sent, created_at, updated_at, completed_at`

func scanSession(row pgx.Row) (domain.Session, error) {
	var s domain.Session
	err := row.Scan(&s.ID, &s.CreatorID, &s.Name, &s.Mode, &s.SectionCount, &s.MaxSectionDurationS,
		&s.Deadline, &s.InviteToken, &s.Status, &s.ReelURL, &s.RetryCount,
		&s.Reminder2hSent, &s.Reminder30mSent, &s.CreatedAt, &s.UpdatedAt, &s.CompletedAt)
	return s, err
}

func scanSessions(rows pgx.Rows) ([]domain.Session, error) {
	defer rows.Close()
	var out []domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *sessionPg) Create(ctx context.Context, session domain.Session, slots []domain.Slot) (domain.Session, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Session{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	created, err := scanSession(tx.QueryRow(ctx, `
		INSERT INTO sessions (creator_id, name, mode, section_count, max_section_duration_s, deadline, invite_token, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+sessionColumns,
		session.CreatorID, session.Name, session.Mode, session.SectionCount, session.MaxSectionDurationS,
		session.Deadline, session.InviteToken, defaultStatus(session.Status),
	))
	if err != nil {
		return domain.Session{}, fmt.Errorf("insert session: %w", err)
	}

	for _, slot := range slots {
		_, err := tx.Exec(ctx, `
			INSERT INTO session_slots (session_id, name, starts_at, ends_at, slot_order)
			VALUES ($1, $2, $3::TIME, $4::TIME, $5)`,
			created.ID, slot.Name, slot.StartsAt.String(), slot.EndsAt.String(), slot.SlotOrder)
		if err != nil {
			return domain.Session{}, fmt.Errorf("insert slot: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Session{}, fmt.Errorf("commit: %w", err)
	}
	return created, nil
}

func defaultStatus(s domain.SessionStatus) domain.SessionStatus {
	if s == "" {
		return domain.SessionStatusActive
	}
	return s
}

func (r *sessionPg) GetByID(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	s, err := scanSession(r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrNotFound
		}
		return domain.Session{}, fmt.Errorf("get session by id: %w", err)
	}
	return s, nil
}

func (r *sessionPg) GetByInviteToken(ctx context.Context, token string) (domain.Session, error) {
	s, err := scanSession(r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE invite_token = $1`, token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrNotFound
		}
		return domain.Session{}, fmt.Errorf("get session by invite_token: %w", err)
	}
	return s, nil
}

func (r *sessionPg) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SessionStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE sessions SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("update session status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *sessionPg) Cancel(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE sessions SET status = 'cancelled', updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("cancel session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// AddParticipant inserts a participant, enforcing the 4-person cap and the
// "one active session per user" rule atomically. Concurrent joins on the same
// session are serialized by a per-session transaction advisory lock; the
// "one active session" check is protected by the unique partial index on
// (user_id) WHERE status = 'active' in participants — we surface the
// race outcome as ErrAlreadyInSession.
func (r *sessionPg) AddParticipant(ctx context.Context, sessionID, userID uuid.UUID, displayName string) (domain.Participant, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Participant{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Advisory lock keyed on the session UUID. hashtext → int4; cast to int8
	// so pg_advisory_xact_lock(bigint) accepts it.
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1::text)::bigint)`, sessionID); err != nil {
		return domain.Participant{}, fmt.Errorf("advisory lock: %w", err)
	}

	// Verify session exists and is active.
	var status domain.SessionStatus
	if err := tx.QueryRow(ctx,
		`SELECT status FROM sessions WHERE id = $1`, sessionID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Participant{}, domain.ErrNotFound
		}
		return domain.Participant{}, fmt.Errorf("load session for join: %w", err)
	}
	if status != domain.SessionStatusActive {
		return domain.Participant{}, domain.ErrSessionNotActive
	}

	// Capacity check.
	var count int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM session_participants WHERE session_id = $1 AND status = 'active'`,
		sessionID).Scan(&count); err != nil {
		return domain.Participant{}, fmt.Errorf("count participants: %w", err)
	}
	if count >= domain.MaxParticipants {
		return domain.Participant{}, domain.ErrSessionFull
	}

	// Already in this session?
	var existsInThis bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM session_participants
			WHERE session_id = $1 AND user_id = $2 AND status = 'active'
		)`, sessionID, userID).Scan(&existsInThis); err != nil {
		return domain.Participant{}, fmt.Errorf("check participant exists: %w", err)
	}
	if existsInThis {
		return domain.Participant{}, domain.ErrAlreadyInSession
	}

	// Already in a different active session?
	var existsElsewhere bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM session_participants
			WHERE user_id = $1 AND status = 'active' AND session_id <> $2
		)`, userID, sessionID).Scan(&existsElsewhere); err != nil {
		return domain.Participant{}, fmt.Errorf("check active session for user: %w", err)
	}
	if existsElsewhere {
		return domain.Participant{}, domain.ErrAlreadyInSession
	}

	var p domain.Participant
	err = tx.QueryRow(ctx, `
		INSERT INTO session_participants (session_id, user_id, display_name_snapshot)
		VALUES ($1, $2, $3)
		RETURNING id, session_id, user_id, display_name_snapshot, joined_at, status`,
		sessionID, userID, displayName,
	).Scan(&p.ID, &p.SessionID, &p.UserID, &p.DisplayNameSnapshot, &p.JoinedAt, &p.Status)
	if err != nil {
		return domain.Participant{}, fmt.Errorf("insert participant: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Participant{}, fmt.Errorf("commit: %w", err)
	}
	return p, nil
}

func (r *sessionPg) GetActiveSessionForUser(ctx context.Context, userID uuid.UUID) (*domain.Session, error) {
	s, err := scanSession(r.pool.QueryRow(ctx, `
		SELECT `+sessionColumns+`
		FROM sessions s
		WHERE s.status = 'active' AND EXISTS (
			SELECT 1 FROM session_participants p
			WHERE p.session_id = s.id AND p.user_id = $1 AND p.status = 'active'
		)
		LIMIT 1`, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active session for user: %w", err)
	}
	return &s, nil
}

func (r *sessionPg) GetSlots(ctx context.Context, sessionID uuid.UUID) ([]domain.Slot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, session_id, name, starts_at::TEXT, ends_at::TEXT, slot_order
		FROM session_slots
		WHERE session_id = $1
		ORDER BY slot_order ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get slots: %w", err)
	}
	defer rows.Close()

	var slots []domain.Slot
	for rows.Next() {
		var s domain.Slot
		var startsAt, endsAt string
		if err := rows.Scan(&s.ID, &s.SessionID, &s.Name, &startsAt, &endsAt, &s.SlotOrder); err != nil {
			return nil, fmt.Errorf("scan slot: %w", err)
		}
		s.StartsAt, err = parseTimeOfDay(startsAt)
		if err != nil {
			return nil, err
		}
		s.EndsAt, err = parseTimeOfDay(endsAt)
		if err != nil {
			return nil, err
		}
		slots = append(slots, s)
	}
	return slots, rows.Err()
}

func parseTimeOfDay(s string) (domain.TimeOfDay, error) {
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n < 2 {
		return domain.TimeOfDay{}, fmt.Errorf("parse time of day %q: %w", s, err)
	}
	return domain.TimeOfDay{Hour: h, Minute: m}, nil
}

func (r *sessionPg) UpsertSlotParticipation(ctx context.Context, slotID, userID uuid.UUID, status domain.SlotParticipationStatus) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO slot_participations (slot_id, user_id, status)
		VALUES ($1, $2, $3)
		ON CONFLICT (slot_id, user_id) DO UPDATE SET status = EXCLUDED.status`,
		slotID, userID, status)
	if err != nil {
		return fmt.Errorf("upsert slot participation: %w", err)
	}
	return nil
}

func (r *sessionPg) GetActiveSessionsPastDeadline(ctx context.Context) ([]domain.Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+sessionColumns+`
		FROM sessions
		WHERE status = 'active' AND deadline <= now()
		ORDER BY deadline ASC`)
	if err != nil {
		return nil, fmt.Errorf("get sessions past deadline: %w", err)
	}
	return scanSessions(rows)
}

func (r *sessionPg) GetSessionsNeedingReminder(ctx context.Context, window time.Duration) ([]domain.Session, error) {
	var flag string
	switch window {
	case 2 * time.Hour:
		flag = "reminder_2h_sent"
	case 30 * time.Minute:
		flag = "reminder_30m_sent"
	default:
		return nil, fmt.Errorf("unsupported reminder window: %s", window)
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM sessions
		WHERE status = 'active'
		  AND %s = false
		  AND deadline > now()
		  AND deadline <= now() + $1::interval
		ORDER BY deadline ASC`, sessionColumns, flag)

	rows, err := r.pool.Query(ctx, query, fmt.Sprintf("%d seconds", int(window.Seconds())))
	if err != nil {
		return nil, fmt.Errorf("get sessions needing reminder: %w", err)
	}
	return scanSessions(rows)
}
