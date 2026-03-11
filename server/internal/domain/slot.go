package domain

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// TimeOfDay represents a wall-clock time without date or timezone.
type TimeOfDay struct {
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
}

func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

func (t TimeOfDay) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *TimeOfDay) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 {
		return fmt.Errorf("invalid time format %q, expected HH:MM", s)
	}

	t.Hour = h
	t.Minute = m
	return t.Validate()
}

func (t TimeOfDay) Validate() error {
	if t.Hour < 0 || t.Hour > 23 {
		return ValidationError("hour must be between 0 and 23")
	}
	if t.Minute < 0 || t.Minute > 59 {
		return ValidationError("minute must be between 0 and 59")
	}
	return nil
}

type Slot struct {
	ID        uuid.UUID `json:"id"`
	SessionID uuid.UUID `json:"session_id"`
	Name      string    `json:"name"`
	StartsAt  TimeOfDay `json:"starts_at"`
	EndsAt    TimeOfDay `json:"ends_at"`
	SlotOrder int       `json:"slot_order"`
}

func (s *Slot) Validate() error {
	if s.Name == "" {
		return ValidationError("slot name must not be empty")
	}
	if err := s.StartsAt.Validate(); err != nil {
		return err
	}
	if err := s.EndsAt.Validate(); err != nil {
		return err
	}
	if s.SlotOrder < 0 {
		return ValidationError("slot_order must be >= 0")
	}
	return nil
}
