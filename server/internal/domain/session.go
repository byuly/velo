package domain

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type SessionMode string

const (
	SessionModeNamedSlots SessionMode = "named_slots"
	SessionModeAutoSlot   SessionMode = "auto_slot"
)

type SessionStatus string

const (
	SessionStatusActive     SessionStatus = "active"
	SessionStatusGenerating SessionStatus = "generating"
	SessionStatusComplete   SessionStatus = "complete"
	SessionStatusFailed     SessionStatus = "failed"
	SessionStatusCancelled  SessionStatus = "cancelled"
)

type ParticipantStatus string

const (
	ParticipantStatusActive   ParticipantStatus = "active"
	ParticipantStatusExcluded ParticipantStatus = "excluded"
)

type SlotParticipationStatus string

const (
	SlotParticipationStatusRecording SlotParticipationStatus = "recording"
	SlotParticipationStatusSkipped   SlotParticipationStatus = "skipped"
)

const (
	MaxSessionNameLength              = 40
	MaxSlotParticipationTitleLength   = 30
	MaxParticipants                   = 4
	MinDeadlineOffset                 = 1 * time.Hour
)

var ValidSectionDurations = map[int]bool{10: true, 15: true, 20: true, 30: true}

type Session struct {
	ID                  uuid.UUID     `json:"id"`
	CreatorID           *uuid.UUID    `json:"creator_id"`
	Name                *string       `json:"name"`
	Mode                SessionMode   `json:"mode"`
	SectionCount        int           `json:"section_count"`
	MaxSectionDurationS int           `json:"max_section_duration_s"`
	Deadline            time.Time     `json:"deadline"`
	InviteToken         string        `json:"invite_token"`
	Status              SessionStatus `json:"status"`
	ReelURL             *string       `json:"reel_url,omitempty"`
	RetryCount          int           `json:"retry_count"`
	Reminder2hSent      bool          `json:"-"`
	Reminder30mSent     bool          `json:"-"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
	CompletedAt         *time.Time    `json:"completed_at,omitempty"`
}

type Participant struct {
	ID                  uuid.UUID         `json:"id"`
	SessionID           uuid.UUID         `json:"session_id"`
	UserID              *uuid.UUID        `json:"user_id"`
	DisplayNameSnapshot string            `json:"display_name_snapshot"`
	JoinedAt            time.Time         `json:"joined_at"`
	Status              ParticipantStatus `json:"status"`
}

type SlotParticipation struct {
	ID     uuid.UUID               `json:"id"`
	SlotID uuid.UUID               `json:"slot_id"`
	UserID uuid.UUID               `json:"user_id"`
	Status SlotParticipationStatus `json:"status"`
	Title  *string                 `json:"title,omitempty"`
}

func (sp *SlotParticipation) Validate() error {
	if sp.Title != nil && utf8.RuneCountInString(*sp.Title) > MaxSlotParticipationTitleLength {
		return ValidationErrorf("title must be at most %d characters", MaxSlotParticipationTitleLength)
	}
	return nil
}

func (s *Session) Validate() error {
	if s.Name != nil && utf8.RuneCountInString(*s.Name) > MaxSessionNameLength {
		return ValidationErrorf("name must be at most %d characters", MaxSessionNameLength)
	}

	if s.Mode != SessionModeNamedSlots {
		return ValidationError("mode must be named_slots")
	}

	if s.SectionCount < 1 || s.SectionCount > 6 {
		return ValidationError("section_count must be between 1 and 6")
	}

	if !ValidSectionDurations[s.MaxSectionDurationS] {
		return ValidationError("max_section_duration_s must be 10, 15, 20, or 30")
	}

	if time.Until(s.Deadline) < MinDeadlineOffset {
		return ValidationError("deadline must be at least 1 hour from now")
	}

	return nil
}
