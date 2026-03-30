package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	queueKey     = "velo:reel_jobs"
	blockTimeout = 5 * time.Second
)

// Compile-time interface check.
var _ JobQueue = (*RedisQueue)(nil)

// RedisQueue implements JobQueue using a Redis LIST (RPUSH/BLPOP).
type RedisQueue struct {
	client *redis.Client
}

// NewRedisQueue creates a queue backed by the given Redis client.
func NewRedisQueue(client *redis.Client) *RedisQueue {
	return &RedisQueue{client: client}
}

// Enqueue appends a session ID to the end of the queue.
func (q *RedisQueue) Enqueue(ctx context.Context, sessionID string) error {
	if err := q.client.RPush(ctx, queueKey, sessionID).Err(); err != nil {
		return fmt.Errorf("enqueue %s: %w", sessionID, err)
	}
	return nil
}

// Dequeue blocks for up to 5 seconds waiting for the next job.
// Returns (nil, nil) on timeout so the caller can check for shutdown.
func (q *RedisQueue) Dequeue(ctx context.Context) (*Job, error) {
	result, err := q.client.BLPop(ctx, blockTimeout, queueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("dequeue: %w", err)
	}

	// BLPop returns [key, value].
	if len(result) < 2 {
		return nil, fmt.Errorf("dequeue: unexpected response length %d", len(result))
	}

	return &Job{SessionID: result[1]}, nil
}
