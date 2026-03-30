package reel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScheduler_GracefulShutdown(t *testing.T) {
	// Cancel context immediately so Run returns without attempting any DB calls.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	sched := &Scheduler{
		store:    &Store{},
		service:  &Service{},
		interval: time.Second,
		log:      testLogger(),
	}

	err := sched.Run(ctx)
	require.NoError(t, err)
}
