package queue

import "context"

// Job represents a reel generation job.
type Job struct {
	SessionID string
}

// JobQueue defines the contract for enqueuing and dequeuing reel jobs.
type JobQueue interface {
	// Enqueue adds a session ID to the job queue.
	Enqueue(ctx context.Context, sessionID string) error

	// Dequeue blocks until a job is available or the context is cancelled.
	// Returns (nil, nil) on timeout (not an error).
	Dequeue(ctx context.Context) (*Job, error)
}
