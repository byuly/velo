package config

import "github.com/caarlos0/env/v11"

type Config struct {
	ServerAddr  string `env:"SERVER_ADDR" envDefault:":8080"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisAddr   string `env:"REDIS_ADDR,required"`
}

func Load() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
