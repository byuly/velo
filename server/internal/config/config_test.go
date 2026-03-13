package config

import (
	"os"
	"testing"
)

func TestLoad_Success(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ServerAddr != ":8080" {
		t.Fatalf("ServerAddr = %q, want %q", cfg.ServerAddr, ":8080")
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.JWTIssuer != "velo" {
		t.Fatalf("JWTIssuer = %q, want %q", cfg.JWTIssuer, "velo")
	}
	if cfg.JWTSecret != "test-jwt-secret" {
		t.Fatalf("JWTSecret = %q, want %q", cfg.JWTSecret, "test-jwt-secret")
	}
	if cfg.AppleAppID != "com.example.velo" {
		t.Fatalf("AppleAppID = %q, want %q", cfg.AppleAppID, "com.example.velo")
	}
	if cfg.CloudFrontDomain != "cdn.example.com" {
		t.Fatalf("CloudFrontDomain = %q, want %q", cfg.CloudFrontDomain, "cdn.example.com")
	}
}

func TestLoad_MissingRequiredEnv(t *testing.T) {
	setRequiredEnv(t)
	unsetEnv(t, "JWT_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing required env error")
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()

	t.Setenv("DATABASE_URL", "postgres://velo:password@localhost:5432/velo?sslmode=disable")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("JWT_SECRET", "test-jwt-secret")
	t.Setenv("APPLE_APP_ID", "com.example.velo")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("S3_CLIPS_BUCKET", "velo-clips")
	t.Setenv("S3_REELS_BUCKET", "velo-reels")
	t.Setenv("CLOUDFRONT_DOMAIN", "cdn.example.com")
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	original, hadValue := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%q) error = %v", key, err)
	}

	t.Cleanup(func() {
		var err error
		if hadValue {
			err = os.Setenv(key, original)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restore env %q error = %v", key, err)
		}
	})
}
