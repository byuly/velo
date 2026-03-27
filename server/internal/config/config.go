package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	ServerAddr  string `env:"SERVER_ADDR" envDefault:":8080"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisAddr   string `env:"REDIS_ADDR,required"`

	// JWTSecret is the HMAC signing key for access tokens. Must be at least 32 bytes.
	JWTSecret string `env:"JWT_SECRET,required"`
	JWTIssuer string `env:"JWT_ISSUER" envDefault:"velo"`

	// Apple
	AppleAppID string `env:"APPLE_APP_ID,required"`

	// AWS / S3
	AWSRegion          string `env:"AWS_REGION,required"`
	AWSAccessKeyID     string `env:"AWS_ACCESS_KEY_ID,required"`
	AWSSecretAccessKey string `env:"AWS_SECRET_ACCESS_KEY,required"`
	S3ClipsBucket      string `env:"S3_CLIPS_BUCKET,required"`
	S3ReelsBucket      string `env:"S3_REELS_BUCKET,required"`
	CloudFrontDomain   string `env:"CLOUDFRONT_DOMAIN,required"`
}

func Load() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, err
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes, got %d", len(cfg.JWTSecret))
	}
	return &cfg, nil
}
