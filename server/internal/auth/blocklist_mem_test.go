package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryBlocklist_BlockAndIsBlocked(t *testing.T) {
	bl := NewMemoryBlocklist(1 * time.Hour) // long sweep — won't fire during test
	defer bl.Stop()
	ctx := context.Background()

	err := bl.Block(ctx, "jti-123", 10*time.Minute)
	require.NoError(t, err)

	blocked, err := bl.IsBlocked(ctx, "jti-123")
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestMemoryBlocklist_IsBlocked_UnknownJTI(t *testing.T) {
	bl := NewMemoryBlocklist(1 * time.Hour)
	defer bl.Stop()
	ctx := context.Background()

	blocked, err := bl.IsBlocked(ctx, "unknown-jti")
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestMemoryBlocklist_TTLExpiry(t *testing.T) {
	bl := NewMemoryBlocklist(1 * time.Hour)
	defer bl.Stop()
	ctx := context.Background()

	// Block with a very short TTL.
	err := bl.Block(ctx, "jti-expire", 1*time.Millisecond)
	require.NoError(t, err)

	// Immediately blocked.
	blocked, err := bl.IsBlocked(ctx, "jti-expire")
	require.NoError(t, err)
	assert.True(t, blocked)

	// Wait past expiry.
	time.Sleep(5 * time.Millisecond)

	blocked, err = bl.IsBlocked(ctx, "jti-expire")
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestMemoryBlocklist_SweepCleansExpired(t *testing.T) {
	bl := NewMemoryBlocklist(10 * time.Millisecond) // fast sweep
	defer bl.Stop()
	ctx := context.Background()

	err := bl.Block(ctx, "jti-sweep", 1*time.Millisecond)
	require.NoError(t, err)

	// Wait for sweep to fire.
	time.Sleep(50 * time.Millisecond)

	bl.mu.Lock()
	_, exists := bl.entries["jti-sweep"]
	bl.mu.Unlock()
	assert.False(t, exists, "expired entry should have been swept")
}
