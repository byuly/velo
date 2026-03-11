package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validSession() *Session {
	return &Session{
		Mode:                SessionModeNamedSlots,
		SectionCount:        3,
		MaxSectionDurationS: 15,
		Deadline:            time.Now().Add(2 * time.Hour),
	}
}

func TestSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(s *Session)
		wantErr string
	}{
		// Valid cases
		{"valid minimal", func(s *Session) {}, ""},
		{"valid with nil name", func(s *Session) { s.Name = nil }, ""},
		{"valid with 40 char name", func(s *Session) { n := strings.Repeat("a", 40); s.Name = &n }, ""},
		{"valid section_count 1", func(s *Session) { s.SectionCount = 1 }, ""},
		{"valid section_count 6", func(s *Session) { s.SectionCount = 6 }, ""},
		{"valid duration 10", func(s *Session) { s.MaxSectionDurationS = 10 }, ""},
		{"valid duration 20", func(s *Session) { s.MaxSectionDurationS = 20 }, ""},
		{"valid duration 30", func(s *Session) { s.MaxSectionDurationS = 30 }, ""},

		// Name
		{"name 41 chars", func(s *Session) { n := strings.Repeat("a", 41); s.Name = &n }, "at most 40"},
		{"name 41 unicode runes", func(s *Session) { n := strings.Repeat("한", 41); s.Name = &n }, "at most 40"},

		// Mode
		{"auto_slot rejected", func(s *Session) { s.Mode = SessionModeAutoSlot }, "named_slots"},
		{"invalid mode", func(s *Session) { s.Mode = "invalid" }, "named_slots"},

		// Section count
		{"section_count 0", func(s *Session) { s.SectionCount = 0 }, "between 1 and 6"},
		{"section_count 7", func(s *Session) { s.SectionCount = 7 }, "between 1 and 6"},
		{"section_count negative", func(s *Session) { s.SectionCount = -1 }, "between 1 and 6"},

		// Duration
		{"duration 5 invalid", func(s *Session) { s.MaxSectionDurationS = 5 }, "10, 15, 20, or 30"},
		{"duration 25 invalid", func(s *Session) { s.MaxSectionDurationS = 25 }, "10, 15, 20, or 30"},

		// Deadline
		{"deadline 30min from now", func(s *Session) { s.Deadline = time.Now().Add(30 * time.Minute) }, "at least 1 hour"},
		{"deadline in the past", func(s *Session) { s.Deadline = time.Now().Add(-1 * time.Hour) }, "at least 1 hour"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := validSession()
			tc.modify(s)
			err := s.Validate()

			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}
