package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		display *string
		wantErr bool
	}{
		{"nil display name", nil, false},
		{"empty display name", ptr(""), false},
		{"40 ascii chars", ptr(strings.Repeat("a", 40)), false},
		{"41 ascii chars", ptr(strings.Repeat("a", 41)), true},
		{"40 unicode runes", ptr(strings.Repeat("한", 40)), false},
		{"41 unicode runes", ptr(strings.Repeat("한", 41)), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &User{DisplayName: tc.display}
			err := u.ValidateUpdate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func ptr(s string) *string { return &s }
