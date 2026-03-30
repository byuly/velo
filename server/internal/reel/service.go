package reel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/byuly/velo/server/internal/ffmpeg"
	"github.com/byuly/velo/server/internal/storage"
	"github.com/google/uuid"
)

// Service orchestrates reel generation for a single session.
type Service struct {
	store       *Store
	storage     storage.Storage
	engine      *ffmpeg.Engine
	composer    *ffmpeg.Composer
	clipsBucket string
	reelsBucket string
	log         *slog.Logger
}

// NewService creates a reel generation Service.
func NewService(
	store *Store,
	storage storage.Storage,
	engine *ffmpeg.Engine,
	composer *ffmpeg.Composer,
	clipsBucket, reelsBucket string,
	log *slog.Logger,
) *Service {
	return &Service{
		store:       store,
		storage:     storage,
		engine:      engine,
		composer:    composer,
		clipsBucket: clipsBucket,
		reelsBucket: reelsBucket,
		log:         log,
	}
}

// Generate produces a reel for the given session.
// It downloads clips from S3, normalizes them, composes the reel, uploads it,
// and updates the session status. The caller is responsible for calling
// FailSession on error.
func (s *Service) Generate(ctx context.Context, sessionID uuid.UUID) error {
	log := s.log.With(slog.String("session_id", sessionID.String()))

	// 1. Fetch all session data.
	data, err := s.store.FetchSessionData(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("fetch session data: %w", err)
	}

	// 2. Create temp work directory.
	workDir, err := os.MkdirTemp("", "velo-reel-*")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := os.MkdirAll(filepath.Join(workDir, "raw"), 0o755); err != nil {
		return fmt.Errorf("create raw dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, "norm"), 0o755); err != nil {
		return fmt.Errorf("create norm dir: %w", err)
	}

	// 3. Run alignment algorithm.
	result := Align(data, workDir)
	if result == nil {
		log.Info("zero submitters — completing with no reel")
		return s.store.CompleteSession(ctx, sessionID, "")
	}

	// 4. Download all clips from S3.
	log.Info("downloading clips", slog.Int("count", len(result.ClipsToFetch)))
	for _, cf := range result.ClipsToFetch {
		if err := s.storage.Download(ctx, s.clipsBucket, cf.S3Key, cf.RawPath); err != nil {
			return fmt.Errorf("download clip %s: %w", cf.S3Key, err)
		}
	}

	// 5. Normalize all clips (single-phase: VFR→CFR at deadline).
	log.Info("normalizing clips", slog.Int("count", len(result.ClipsToFetch)))
	for _, cf := range result.ClipsToFetch {
		if err := s.composer.NormalizeClip(ctx, cf.RawPath, cf.NormPath); err != nil {
			return fmt.Errorf("normalize clip %s: %w", cf.S3Key, err)
		}
	}

	// 6. Compose the reel.
	log.Info("composing reel")
	reelPath, err := s.engine.Compose(ctx, result.Request)
	if err != nil {
		return fmt.Errorf("compose reel: %w", err)
	}

	// 7. Upload to S3.
	reelKey := fmt.Sprintf("reels/%s/reel.mp4", sessionID)
	log.Info("uploading reel", slog.String("key", reelKey))
	if err := s.storage.Upload(ctx, s.reelsBucket, reelKey, reelPath); err != nil {
		return fmt.Errorf("upload reel: %w", err)
	}

	// 8. Update session.
	reelURL := s.storage.ReelURL(reelKey)
	if err := s.store.CompleteSession(ctx, sessionID, reelURL); err != nil {
		return fmt.Errorf("complete session: %w", err)
	}

	log.Info("reel generation complete", slog.String("reel_url", reelURL))
	return nil
}
