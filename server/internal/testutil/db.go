package testutil

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	containerOnce sync.Once
	containerDSN  string
	containerErr  error
)

func startContainer() (string, error) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("velo_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return "", fmt.Errorf("start postgres container: %w", err)
	}

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("get connection string: %w", err)
	}

	return dsn, nil
}

func migrationsPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

// SetupTestDB creates an isolated database for a single test.
// It starts a shared Postgres container (once per package), creates a unique
// database, applies migrations, and returns a pool. The database is dropped
// on test cleanup.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	containerOnce.Do(func() {
		containerDSN, containerErr = startContainer()
	})
	if containerErr != nil {
		t.Fatalf("postgres container: %v", containerErr)
	}

	ctx := context.Background()

	// Connect to the default database to create a new one.
	adminPool, err := pgxpool.New(ctx, containerDSN)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}

	// Unique database name per test.
	sanitized := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dbName := fmt.Sprintf("test_%s_%d", sanitized, time.Now().UnixNano())
	// Postgres identifiers max 63 chars.
	if len(dbName) > 63 {
		dbName = dbName[:63]
	}

	_, err = adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %q", dbName))
	if err != nil {
		adminPool.Close()
		t.Fatalf("create database: %v", err)
	}
	adminPool.Close()

	// Build DSN for the new database by replacing the path in the URL.
	testDSN, err := replaceDBInDSN(containerDSN, dbName)
	if err != nil {
		t.Fatalf("build test dsn: %v", err)
	}

	// Apply migrations.
	migPath := migrationsPath()
	migDSN := strings.Replace(testDSN, "postgres://", "pgx5://", 1)
	m, err := migrate.New("file://"+migPath, migDSN)
	if err != nil {
		t.Fatalf("migrate new: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate up: %v", err)
	}
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		t.Fatalf("migrate close src: %v", srcErr)
	}
	if dbErr != nil {
		t.Fatalf("migrate close db: %v", dbErr)
	}

	// Connect to the test database.
	pool, err := pgxpool.New(ctx, testDSN)
	if err != nil {
		t.Fatalf("test pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()

		// Drop the test database.
		admin, err := pgxpool.New(context.Background(), containerDSN)
		if err != nil {
			return
		}
		defer admin.Close()
		admin.Exec(context.Background(), fmt.Sprintf("DROP DATABASE %q WITH (FORCE)", dbName))
	})

	return pool
}

func replaceDBInDSN(dsn, dbName string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	u.Path = "/" + dbName
	return u.String(), nil
}
