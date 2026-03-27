package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/queue"
	"github.com/byuly/velo/server/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestEnqueueDequeue(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	q := queue.NewRedisQueue(rdb)
	ctx := context.Background()

	require.NoError(t, q.Enqueue(ctx, "session-abc"))

	job, err := q.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, "session-abc", job.SessionID)
}

func TestFIFO(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	q := queue.NewRedisQueue(rdb)
	ctx := context.Background()

	ids := []string{"first", "second", "third"}
	for _, id := range ids {
		require.NoError(t, q.Enqueue(ctx, id))
	}

	for _, expected := range ids {
		job, err := q.Dequeue(ctx)
		require.NoError(t, err)
		require.NotNil(t, job)
		require.Equal(t, expected, job.SessionID)
	}
}

func TestDequeueEmpty(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	q := queue.NewRedisQueue(rdb)

	// Use a short-lived context so we don't wait the full 5s BLPOP timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	job, err := q.Dequeue(ctx)
	// Context timeout triggers before BLPOP timeout — returns error.
	// Either nil job or context error is acceptable for empty queue.
	if err != nil {
		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
		return
	}
	require.Nil(t, job, "empty queue should return nil job")
}

func TestDequeueContextCancelled(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	q := queue.NewRedisQueue(rdb)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := q.Dequeue(ctx)
	require.Error(t, err)
}
