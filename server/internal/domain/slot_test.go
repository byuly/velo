package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeOfDay_String(t *testing.T) {
	tests := []struct {
		tod  TimeOfDay
		want string
	}{
		{TimeOfDay{0, 0}, "00:00"},
		{TimeOfDay{9, 5}, "09:05"},
		{TimeOfDay{23, 59}, "23:59"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, tc.tod.String())
	}
}

func TestTimeOfDay_MarshalJSON(t *testing.T) {
	tod := TimeOfDay{14, 30}
	data, err := json.Marshal(tod)
	require.NoError(t, err)
	assert.Equal(t, `"14:30"`, string(data))
}

func TestTimeOfDay_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    TimeOfDay
		wantErr bool
	}{
		{"valid", `"09:30"`, TimeOfDay{9, 30}, false},
		{"midnight", `"00:00"`, TimeOfDay{0, 0}, false},
		{"invalid format", `"abc"`, TimeOfDay{}, true},
		{"hour out of range", `"25:00"`, TimeOfDay{}, true},
		{"minute out of range", `"12:61"`, TimeOfDay{}, true},
		{"not a string", `123`, TimeOfDay{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tod TimeOfDay
			err := json.Unmarshal([]byte(tc.input), &tod)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, tod)
			}
		})
	}
}

func TestTimeOfDay_Validate(t *testing.T) {
	tests := []struct {
		name    string
		tod     TimeOfDay
		wantErr bool
	}{
		{"valid", TimeOfDay{12, 30}, false},
		{"min boundary", TimeOfDay{0, 0}, false},
		{"max boundary", TimeOfDay{23, 59}, false},
		{"hour negative", TimeOfDay{-1, 0}, true},
		{"hour 24", TimeOfDay{24, 0}, true},
		{"minute negative", TimeOfDay{12, -1}, true},
		{"minute 60", TimeOfDay{12, 60}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tod.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSlot_Validate(t *testing.T) {
	valid := func() *Slot {
		return &Slot{
			Name:      "Morning",
			StartsAt:  TimeOfDay{9, 0},
			EndsAt:    TimeOfDay{10, 0},
			SlotOrder: 0,
		}
	}

	tests := []struct {
		name    string
		modify  func(s *Slot)
		wantErr bool
	}{
		{"valid", func(s *Slot) {}, false},
		{"empty name", func(s *Slot) { s.Name = "" }, true},
		{"invalid starts_at", func(s *Slot) { s.StartsAt = TimeOfDay{25, 0} }, true},
		{"invalid ends_at", func(s *Slot) { s.EndsAt = TimeOfDay{12, 60} }, true},
		{"negative slot_order", func(s *Slot) { s.SlotOrder = -1 }, true},
		{"zero slot_order", func(s *Slot) { s.SlotOrder = 0 }, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := valid()
			tc.modify(s)
			err := s.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
