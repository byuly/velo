package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const blocklistKeyPrefix = "blocklist:jti:"

// TokenBlocklist manages revoked access tokens by JTI.
type TokenBlocklist interface {
	Block(ctx context.Context, jti string, ttl time.Duration) error
	IsBlocked(ctx context.Context, jti string) (bool, error)
}

// RedisBlocklist implements TokenBlocklist using Redis SET with TTL.
type RedisBlocklist struct {
	client *redis.Client
}

func NewRedisBlocklist(client *redis.Client) *RedisBlocklist {
	return &RedisBlocklist{client: client}
}

func (r *RedisBlocklist) Block(ctx context.Context, jti string, ttl time.Duration) error {
	return r.client.Set(ctx, blocklistKeyPrefix+jti, "1", ttl).Err()
}

func (r *RedisBlocklist) IsBlocked(ctx context.Context, jti string) (bool, error) {
	n, err := r.client.Exists(ctx, blocklistKeyPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
