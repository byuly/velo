package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/byuly/velo/server/internal/config"
	"github.com/byuly/velo/server/internal/ffmpeg"
	apphandler "github.com/byuly/velo/server/internal/handler"
	mw "github.com/byuly/velo/server/internal/middleware"
	"github.com/byuly/velo/server/internal/reel"
	"github.com/byuly/velo/server/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()

	// --- Reel generation pipeline ---
	var wg sync.WaitGroup

	s3Client, err := storage.NewS3Client(ctx, cfg.AWSRegion, cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.CloudFrontDomain)
	if err != nil {
		log.Error("failed to create S3 client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	composer, err := ffmpeg.New()
	if err != nil {
		log.Warn("ffmpeg unavailable — reel generation disabled", slog.String("error", err.Error()))
	}

	if composer != nil {
		engine := ffmpeg.NewEngine(composer)
		reelStore := reel.NewStore(db)
		reelService := reel.NewService(reelStore, s3Client, engine, composer, cfg.S3ClipsBucket, cfg.S3ReelsBucket, log)
		reelScheduler := reel.NewScheduler(reelStore, reelService, log)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := reelScheduler.Run(ctx); err != nil {
				log.Error("reel scheduler error", slog.String("error", err.Error()))
			}
		}()
	}

	// --- Auth ---
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer)
	blocklist := auth.NewRedisBlocklist(rdb)
	authHandler := apphandler.NewAuthHandler(jwtManager, blocklist)

	// --- HTTP server ---
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(mw.Logger(log))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := db.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "detail": "database unreachable"})
			return
		}
		if err := rdb.Ping(r.Context()).Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "detail": "redis unreachable"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Route("/auth", func(r chi.Router) {
		r.Use(mw.Auth(jwtManager, blocklist))
		r.Post("/logout", authHandler.Logout)
	})

	srv := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: r,
	}

	go func() {
		log.Info("api server starting", slog.String("addr", cfg.ServerAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", slog.String("error", err.Error()))
	}

	// Wait for in-progress reel generation to finish.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		log.Info("reel scheduler stopped")
	case <-time.After(120 * time.Second):
		log.Warn("reel scheduler shutdown timeout")
	}
}
