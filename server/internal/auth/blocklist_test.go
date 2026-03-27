package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBlocklist(t *testing.T) (*RedisBlocklist, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return NewRedisBlocklist(rdb), mr
}

func TestRedisBlocklist_BlockAndIsBlocked(t *testing.T) {
	bl, _ := setupBlocklist(t)
	ctx := context.Background()

	err := bl.Block(ctx, "jti-123", 10*time.Minute)
	require.NoError(t, err)

	blocked, err := bl.IsBlocked(ctx, "jti-123")
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestRedisBlocklist_IsBlocked_UnknownJTI(t *testing.T) {
	bl, _ := setupBlocklist(t)
	ctx := context.Background()

	blocked, err := bl.IsBlocked(ctx, "unknown-jti")
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestRedisBlocklist_TTLExpiry(t *testing.T) {
	bl, mr := setupBlocklist(t)
	ctx := context.Background()

	err := bl.Block(ctx, "jti-expire", 1*time.Second)
	require.NoError(t, err)

	blocked, err := bl.IsBlocked(ctx, "jti-expire")
	require.NoError(t, err)
	assert.True(t, blocked)

	mr.FastForward(2 * time.Second)

	blocked, err = bl.IsBlocked(ctx, "jti-expire")
	require.NoError(t, err)
	assert.False(t, blocked)
}
