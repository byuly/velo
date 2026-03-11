package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClip_Validate(t *testing.T) {
	valid := func() *Clip {
		return &Clip{
			S3Key:      "clips/abc/123.m4a",
			DurationMs: 5000,
			RecordedAt: time.Now(),
		}
	}

	tests := []struct {
		name    string
		modify  func(c *Clip)
		wantErr bool
	}{
		{"valid", func(c *Clip) {}, false},
		{"empty s3_key", func(c *Clip) { c.S3Key = "" }, true},
		{"zero duration", func(c *Clip) { c.DurationMs = 0 }, true},
		{"negative duration", func(c *Clip) { c.DurationMs = -1 }, true},
		{"zero recorded_at", func(c *Clip) { c.RecordedAt = time.Time{} }, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.modify(c)
			err := c.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
