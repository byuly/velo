package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/byuly/velo/server/internal/config"
	"github.com/byuly/velo/server/internal/ffmpeg"
	dbmigrate "github.com/byuly/velo/server/internal/migrate"
	"github.com/byuly/velo/server/internal/reel"
	"github.com/byuly/velo/server/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	var handler slog.Handler
	if cfg.LogLevel == "debug" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	log := slog.New(handler)
	slog.SetDefault(log)

	log.Info("reel worker starting", slog.String("version", version))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// --- Auto-migrate ---
	migPath := "/migrations"
	if _, err := os.Stat(migPath); os.IsNotExist(err) {
		migPath = "migrations"
	}
	if err := dbmigrate.Up(cfg.DatabaseURL, migPath, log); err != nil {
		log.Error("auto-migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// --- Reel pipeline ---
	s3Client, err := storage.NewS3Client(ctx, cfg.AWSRegion, cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.CloudFrontDomain)
	if err != nil {
		log.Error("failed to create S3 client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	composer, err := ffmpeg.New()
	if err != nil {
		log.Error("ffmpeg unavailable — cannot generate reels", slog.String("error", err.Error()))
		os.Exit(1)
	}

	engine := ffmpeg.NewEngine(composer)
	reelStore := reel.NewStore(db)
	reelService := reel.NewService(reelStore, s3Client, engine, composer, cfg.S3ClipsBucket, cfg.S3ReelsBucket, log)
	scheduler := reel.NewScheduler(reelStore, reelService, log)

	// Single pass: claim due sessions, process, exit.
	if err := scheduler.RunOnce(ctx); err != nil {
		log.Error("worker failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("reel worker finished")
}
