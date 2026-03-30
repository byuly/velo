package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T { return &v }

// coalesce returns the override value if non-nil, otherwise the default.
func coalesce[T any](override *T, def T) T {
	if override != nil {
		return *override
	}
	return def
}

// --- User ---

type UserOverrides struct {
	AppleSub    *string
	DisplayName **string
	AvatarURL   **string
	APNsToken   **string
}

func CreateUser(t *testing.T, pool *pgxpool.Pool, overrides *UserOverrides) domain.User {
	t.Helper()

	appleSub := uuid.New().String()
	var displayName, avatarURL, apnsToken *string

	if overrides != nil {
		appleSub = coalesce(overrides.AppleSub, appleSub)
		if overrides.DisplayName != nil {
			displayName = *overrides.DisplayName
		}
		if overrides.AvatarURL != nil {
			avatarURL = *overrides.AvatarURL
		}
		if overrides.APNsToken != nil {
			apnsToken = *overrides.APNsToken
		}
	}

	var u domain.User
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (apple_sub, display_name, avatar_url, apns_token)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, apple_sub, display_name, avatar_url, apns_token, created_at, updated_at`,
		appleSub, displayName, avatarURL, apnsToken,
	).Scan(&u.ID, &u.AppleSub, &u.DisplayName, &u.AvatarURL, &u.APNsToken, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

// --- Session ---

type SessionOverrides struct {
	Name                **string
	Mode                *domain.SessionMode
	SectionCount        *int
	MaxSectionDurationS *int
	Deadline            *time.Time
	InviteToken         *string
	Status              *domain.SessionStatus
}

func CreateSession(t *testing.T, pool *pgxpool.Pool, creatorID uuid.UUID, overrides *SessionOverrides) domain.Session {
	t.Helper()

	mode := domain.SessionModeNamedSlots
	sectionCount := 3
	maxDur := 15
	deadline := time.Now().Add(24 * time.Hour)
	inviteToken := uuid.New().String()
	status := domain.SessionStatusActive
	var name *string

	if overrides != nil {
		mode = coalesce(overrides.Mode, mode)
		sectionCount = coalesce(overrides.SectionCount, sectionCount)
		maxDur = coalesce(overrides.MaxSectionDurationS, maxDur)
		deadline = coalesce(overrides.Deadline, deadline)
		inviteToken = coalesce(overrides.InviteToken, inviteToken)
		status = coalesce(overrides.Status, status)
		if overrides.Name != nil {
			name = *overrides.Name
		}
	}

	var s domain.Session
	err := pool.QueryRow(context.Background(),
		`INSERT INTO sessions (creator_id, name, mode, section_count, max_section_duration_s, deadline, invite_token, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, creator_id, name, mode, section_count, max_section_duration_s, deadline, invite_token, status,
		           reel_url, retry_count, reminder_2h_sent, reminder_30m_sent, created_at, updated_at, completed_at`,
		creatorID, name, mode, sectionCount, maxDur, deadline, inviteToken, status,
	).Scan(&s.ID, &s.CreatorID, &s.Name, &s.Mode, &s.SectionCount, &s.MaxSectionDurationS, &s.Deadline,
		&s.InviteToken, &s.Status, &s.ReelURL, &s.RetryCount, &s.Reminder2hSent, &s.Reminder30mSent,
		&s.CreatedAt, &s.UpdatedAt, &s.CompletedAt)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return s
}

// --- Slot ---

type SlotOverrides struct {
	Name      *string
	StartsAt  *string // "HH:MM" format for SQL TIME
	EndsAt    *string
	SlotOrder *int
}

func CreateSlot(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, overrides *SlotOverrides) domain.Slot {
	t.Helper()

	name := "Default Slot"
	startsAt := "09:00"
	endsAt := "10:00"
	slotOrder := 0

	if overrides != nil {
		name = coalesce(overrides.Name, name)
		startsAt = coalesce(overrides.StartsAt, startsAt)
		endsAt = coalesce(overrides.EndsAt, endsAt)
		slotOrder = coalesce(overrides.SlotOrder, slotOrder)
	}

	var s domain.Slot
	var startsAtStr, endsAtStr string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO session_slots (session_id, name, starts_at, ends_at, slot_order)
		 VALUES ($1, $2, $3::TIME, $4::TIME, $5)
		 RETURNING id, session_id, name, starts_at::TEXT, ends_at::TEXT, slot_order`,
		sessionID, name, startsAt, endsAt, slotOrder,
	).Scan(&s.ID, &s.SessionID, &s.Name, &startsAtStr, &endsAtStr, &s.SlotOrder)
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	s.StartsAt = parseTimeOfDay(t, startsAtStr)
	s.EndsAt = parseTimeOfDay(t, endsAtStr)
	return s
}

func parseTimeOfDay(t *testing.T, s string) domain.TimeOfDay {
	t.Helper()
	var h, m int
	_, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil {
		t.Fatalf("parse time of day %q: %v", s, err)
	}
	return domain.TimeOfDay{Hour: h, Minute: m}
}

// --- Participant ---

type ParticipantOverrides struct {
	DisplayNameSnapshot *string
	Status              *domain.ParticipantStatus
}

func CreateParticipant(t *testing.T, pool *pgxpool.Pool, sessionID, userID uuid.UUID, overrides *ParticipantOverrides) domain.Participant {
	t.Helper()

	snapshot := "Test User"
	status := domain.ParticipantStatusActive

	if overrides != nil {
		snapshot = coalesce(overrides.DisplayNameSnapshot, snapshot)
		status = coalesce(overrides.Status, status)
	}

	var p domain.Participant
	err := pool.QueryRow(context.Background(),
		`INSERT INTO session_participants (session_id, user_id, display_name_snapshot, status)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, session_id, user_id, display_name_snapshot, joined_at, status`,
		sessionID, userID, snapshot, status,
	).Scan(&p.ID, &p.SessionID, &p.UserID, &p.DisplayNameSnapshot, &p.JoinedAt, &p.Status)
	if err != nil {
		t.Fatalf("create participant: %v", err)
	}
	return p
}

// --- Clip ---

type ClipOverrides struct {
	S3Key      *string
	RecordedAt *time.Time
	DurationMs *int
	SlotID     **uuid.UUID
}

func CreateClip(t *testing.T, pool *pgxpool.Pool, sessionID, userID uuid.UUID, overrides *ClipOverrides) domain.Clip {
	t.Helper()

	s3Key := "clips/" + uuid.New().String() + ".m4a"
	recordedAt := time.Now()
	arrivedAt := time.Now()
	durationMs := 5000
	var slotID *uuid.UUID

	if overrides != nil {
		s3Key = coalesce(overrides.S3Key, s3Key)
		recordedAt = coalesce(overrides.RecordedAt, recordedAt)
		durationMs = coalesce(overrides.DurationMs, durationMs)
		if overrides.SlotID != nil {
			slotID = *overrides.SlotID
		}
	}

	var c domain.Clip
	err := pool.QueryRow(context.Background(),
		`INSERT INTO clips (session_id, user_id, slot_id, s3_key, recorded_at, arrived_at, duration_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, session_id, user_id, slot_id, s3_key, recorded_at, arrived_at, recorded_at_clamped, duration_ms, created_at`,
		sessionID, userID, slotID, s3Key, recordedAt, arrivedAt, durationMs,
	).Scan(&c.ID, &c.SessionID, &c.UserID, &c.SlotID, &c.S3Key, &c.RecordedAt, &c.ArrivedAt,
		&c.RecordedAtClamped, &c.DurationMs, &c.CreatedAt)
	if err != nil {
		t.Fatalf("create clip: %v", err)
	}
	return c
}

// --- SlotParticipation ---

type SlotParticipationOverrides struct {
	Status *domain.SlotParticipationStatus
	Title  **string
}

func CreateSlotParticipation(t *testing.T, pool *pgxpool.Pool, slotID, userID uuid.UUID, overrides *SlotParticipationOverrides) domain.SlotParticipation {
	t.Helper()

	status := domain.SlotParticipationStatusRecording
	var title *string

	if overrides != nil {
		status = coalesce(overrides.Status, status)
		if overrides.Title != nil {
			title = *overrides.Title
		}
	}

	var sp domain.SlotParticipation
	err := pool.QueryRow(context.Background(),
		`INSERT INTO slot_participations (slot_id, user_id, status, title)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, slot_id, user_id, status, title`,
		slotID, userID, status, title,
	).Scan(&sp.ID, &sp.SlotID, &sp.UserID, &sp.Status, &sp.Title)
	if err != nil {
		t.Fatalf("create slot participation: %v", err)
	}
	return sp
}

// --- RefreshToken ---

type RefreshTokenOverrides struct {
	TokenHash *string
	ExpiresAt *time.Time
}

func CreateRefreshToken(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, overrides *RefreshTokenOverrides) domain.RefreshToken {
	t.Helper()

	tokenHash := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	if overrides != nil {
		tokenHash = coalesce(overrides.TokenHash, tokenHash)
		expiresAt = coalesce(overrides.ExpiresAt, expiresAt)
	}

	var rt domain.RefreshToken
	err := pool.QueryRow(context.Background(),
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, token_hash, expires_at, created_at`,
		userID, tokenHash, expiresAt,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.CreatedAt)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}
	return rt
}
