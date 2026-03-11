package domain

import (
	"time"

	"github.com/google/uuid"
)

const ClipTimestampTolerance = 30 * time.Minute

type Clip struct {
	ID               uuid.UUID  `json:"id"`
	SessionID        uuid.UUID  `json:"session_id"`
	UserID           *uuid.UUID `json:"user_id"`
	SlotID           *uuid.UUID `json:"slot_id"`
	S3Key            string     `json:"s3_key"`
	RecordedAt       time.Time  `json:"recorded_at"`
	ArrivedAt        time.Time  `json:"arrived_at"`
	RecordedAtClamped bool      `json:"recorded_at_clamped"`
	DurationMs       int        `json:"duration_ms"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (c *Clip) Validate() error {
	if c.S3Key == "" {
		return ValidationError("s3_key must not be empty")
	}
	if c.DurationMs <= 0 {
		return ValidationError("duration_ms must be greater than 0")
	}
	if c.RecordedAt.IsZero() {
		return ValidationError("recorded_at must not be zero")
	}
	return nil
}
