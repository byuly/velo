package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	redisOnce sync.Once
	redisAddr string
	redisErr  error
	redisDB   atomic.Int32 // incremented per test for isolation
)

func startRedisContainer() (string, error) {
	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return "", fmt.Errorf("start redis container: %w", err)
	}

	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		return "", fmt.Errorf("get redis endpoint: %w", err)
	}

	return endpoint, nil
}

// SetupTestRedis returns a Redis client connected to a shared test container.
// Each call selects a unique Redis DB number (0-15) so tests are isolated
// without relying on FlushDB, making it safe for parallel use.
// Skips the test if Docker is not available.
func SetupTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker not available — skipping")
	}

	redisOnce.Do(func() {
		redisAddr, redisErr = startRedisContainer()
	})
	if redisErr != nil {
		t.Fatalf("redis container: %v", redisErr)
	}

	db := int(redisDB.Add(1))
	client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: db})

	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})

	return client
}
