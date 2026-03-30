package reel

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxRetries is the maximum number of reel generation attempts before giving up.
const MaxRetries = 3

// SessionData holds all denormalized data needed to generate one reel.
type SessionData struct {
	Session        SessionRow
	Slots          []SlotRow
	Participants   []ParticipantRow
	Clips          []ClipRow
	Participations []ParticipationRow
}

// SessionRow holds the session fields needed for reel generation.
type SessionRow struct {
	ID                  uuid.UUID
	CreatorID           *uuid.UUID
	MaxSectionDurationS int
	RetryCount          int
}

// SlotRow holds slot metadata.
type SlotRow struct {
	ID        uuid.UUID
	SlotOrder int
	Name      string
}

// ParticipantRow holds participant metadata.
type ParticipantRow struct {
	UserID              uuid.UUID
	DisplayNameSnapshot string
	JoinedAt            time.Time
	Status              string
}

// ClipRow holds clip metadata needed for alignment.
type ClipRow struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	SlotID     uuid.UUID
	S3Key      string
	RecordedAt time.Time
	DurationMs int
}

// ParticipationRow holds slot participation metadata.
type ParticipationRow struct {
	SlotID uuid.UUID
	UserID uuid.UUID
	Status string
	Title  *string
}

// Store provides database operations for reel generation.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a reel Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// ClaimDueSessions atomically finds active sessions past their deadline
// and transitions them to 'generating'. Returns claimed session IDs.
func (s *Store) ClaimDueSessions(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE sessions
		SET    status = 'generating', updated_at = now()
		WHERE  id IN (
			SELECT id FROM sessions
			WHERE  status = 'active'
			  AND  deadline <= now()
			  AND  retry_count < $1
			ORDER BY deadline
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id
	`, MaxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("claim due sessions: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan claimed session: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FetchSessionData loads all data needed to generate a session's reel.
func (s *Store) FetchSessionData(ctx context.Context, sessionID uuid.UUID) (*SessionData, error) {
	var data SessionData

	// 1. Session
	err := s.pool.QueryRow(ctx, `
		SELECT id, creator_id, max_section_duration_s, retry_count
		FROM sessions WHERE id = $1
	`, sessionID).Scan(
		&data.Session.ID,
		&data.Session.CreatorID,
		&data.Session.MaxSectionDurationS,
		&data.Session.RetryCount,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch session %s: %w", sessionID, err)
	}

	// 2. Slots
	slotRows, err := s.pool.Query(ctx, `
		SELECT id, slot_order, name
		FROM session_slots WHERE session_id = $1
		ORDER BY slot_order
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("fetch slots: %w", err)
	}
	data.Slots, err = pgx.CollectRows(slotRows, func(row pgx.CollectableRow) (SlotRow, error) {
		var r SlotRow
		err := row.Scan(&r.ID, &r.SlotOrder, &r.Name)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan slots: %w", err)
	}

	// 3. Participants
	partRows, err := s.pool.Query(ctx, `
		SELECT user_id, display_name_snapshot, joined_at, status
		FROM session_participants WHERE session_id = $1
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("fetch participants: %w", err)
	}
	data.Participants, err = pgx.CollectRows(partRows, func(row pgx.CollectableRow) (ParticipantRow, error) {
		var r ParticipantRow
		err := row.Scan(&r.UserID, &r.DisplayNameSnapshot, &r.JoinedAt, &r.Status)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan participants: %w", err)
	}

	// 4. Clips
	clipRows, err := s.pool.Query(ctx, `
		SELECT id, user_id, slot_id, s3_key, recorded_at, duration_ms
		FROM clips WHERE session_id = $1
		ORDER BY slot_id, user_id, recorded_at
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("fetch clips: %w", err)
	}
	data.Clips, err = pgx.CollectRows(clipRows, func(row pgx.CollectableRow) (ClipRow, error) {
		var r ClipRow
		err := row.Scan(&r.ID, &r.UserID, &r.SlotID, &r.S3Key, &r.RecordedAt, &r.DurationMs)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan clips: %w", err)
	}

	// 5. Participations
	spRows, err := s.pool.Query(ctx, `
		SELECT sp.slot_id, sp.user_id, sp.status, sp.title
		FROM slot_participations sp
		JOIN session_slots ss ON ss.id = sp.slot_id
		WHERE ss.session_id = $1
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("fetch participations: %w", err)
	}
	data.Participations, err = pgx.CollectRows(spRows, func(row pgx.CollectableRow) (ParticipationRow, error) {
		var r ParticipationRow
		err := row.Scan(&r.SlotID, &r.UserID, &r.Status, &r.Title)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan participations: %w", err)
	}

	return &data, nil
}

// CompleteSession marks a session as complete with the given reel URL.
func (s *Store) CompleteSession(ctx context.Context, sessionID uuid.UUID, reelURL string) error {
	var url *string
	if reelURL != "" {
		url = &reelURL
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET    status = 'complete', reel_url = $2, completed_at = now(), updated_at = now()
		WHERE  id = $1
	`, sessionID, url)
	if err != nil {
		return fmt.Errorf("complete session %s: %w", sessionID, err)
	}
	return nil
}

// FailSession increments the retry count and either re-queues (back to active)
// or marks as failed if retries are exhausted.
func (s *Store) FailSession(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET    status = CASE WHEN retry_count + 1 < $2 THEN 'active'::session_status ELSE 'failed'::session_status END,
		       retry_count = retry_count + 1,
		       updated_at = now()
		WHERE  id = $1
	`, sessionID, MaxRetries)
	if err != nil {
		return fmt.Errorf("fail session %s: %w", sessionID, err)
	}
	return nil
}
