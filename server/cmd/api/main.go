package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/byuly/velo/server/internal/auth"
	"github.com/byuly/velo/server/internal/config"
	dbmigrate "github.com/byuly/velo/server/internal/migrate"
	apphandler "github.com/byuly/velo/server/internal/handler"
	mw "github.com/byuly/velo/server/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var version = "dev" // set via -ldflags at build time

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

	// --- Auto-migrate ---
	migPath := "/migrations"
	if _, err := os.Stat(migPath); os.IsNotExist(err) {
		migPath = "migrations" // local dev: relative path
	}
	if err := dbmigrate.Up(cfg.DatabaseURL, migPath, log); err != nil {
		log.Error("auto-migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// --- Token blocklist (Redis or in-memory) ---
	var blocklist auth.TokenBlocklist
	var rdb *redis.Client

	if cfg.RedisAddr != "" {
		rdb = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
		defer rdb.Close()
		blocklist = auth.NewRedisBlocklist(rdb)
		log.Info("using Redis token blocklist", slog.String("addr", cfg.RedisAddr))
	} else {
		memBl := auth.NewMemoryBlocklist(5 * time.Minute)
		defer memBl.Stop()
		blocklist = memBl
		log.Info("using in-memory token blocklist (Redis not configured)")
	}

	// --- Auth ---
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer)
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
		resp := map[string]string{"status": "ok"}

		if err := db.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "detail": "database unreachable"})
			return
		}
		if rdb != nil {
			if err := rdb.Ping(r.Context()).Err(); err != nil {
				resp["redis"] = "unreachable"
			}
		}
		json.NewEncoder(w).Encode(resp)
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
		log.Info("api server starting", slog.String("addr", cfg.ServerAddr), slog.String("version", version))
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
}
