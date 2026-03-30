package reel

import (
	"context"
	"log/slog"
	"time"
)

const (
	// DefaultPollInterval is how often the scheduler checks for due sessions.
	DefaultPollInterval = 30 * time.Second

	// DefaultClaimLimit is the max sessions to claim per poll cycle.
	// Set to 1 for serial processing (CPU-bound FFmpeg on a single instance).
	DefaultClaimLimit = 1
)

// Scheduler polls Postgres for sessions past their deadline and generates reels.
type Scheduler struct {
	store    *Store
	service  *Service
	interval time.Duration
	log      *slog.Logger
}

// NewScheduler creates a Scheduler.
func NewScheduler(store *Store, service *Service, log *slog.Logger) *Scheduler {
	return &Scheduler{
		store:    store,
		service:  service,
		interval: DefaultPollInterval,
		log:      log,
	}
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	s.log.Info("reel scheduler started", slog.Duration("interval", s.interval))

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Poll immediately on startup, then on each tick.
	for {
		select {
		case <-ctx.Done():
			s.log.Info("reel scheduler shutting down")
			return nil
		default:
		}

		s.poll(ctx)

		select {
		case <-ctx.Done():
			s.log.Info("reel scheduler shutting down")
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) poll(ctx context.Context) {
	ids, err := s.store.ClaimDueSessions(ctx, DefaultClaimLimit)
	if err != nil {
		s.log.Error("claim due sessions", slog.String("error", err.Error()))
		return
	}

	for _, id := range ids {
		log := s.log.With(slog.String("session_id", id.String()))
		log.Info("generating reel")

		if err := s.service.Generate(ctx, id); err != nil {
			log.Error("reel generation failed", slog.String("error", err.Error()))
			if failErr := s.store.FailSession(ctx, id); failErr != nil {
				log.Error("failed to update session status", slog.String("error", failErr.Error()))
			}
			continue
		}

		log.Info("reel generation complete")
	}
}
