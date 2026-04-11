package migrate

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Up runs all pending migrations from the given directory against the database.
// Returns nil if the schema is already up to date.
func Up(databaseURL, migrationsPath string, log *slog.Logger) error {
	migURL := strings.Replace(databaseURL, "postgres://", "pgx5://", 1)

	m, err := migrate.New("file://"+migrationsPath, migURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	version, dirty, _ := m.Version()
	log.Info("migration status", slog.Uint64("current_version", uint64(version)), slog.Bool("dirty", dirty))

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}

	newVersion, _, _ := m.Version()
	if newVersion != version {
		log.Info("migrations applied", slog.Uint64("new_version", uint64(newVersion)))
	} else {
		log.Info("database schema up to date")
	}

	return nil
}
