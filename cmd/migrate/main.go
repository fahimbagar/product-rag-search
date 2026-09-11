// Command migrate applies (or rolls back) SQL migrations against the
// configured Postgres database.
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "migrations"
	}

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	m, err := migrate.New(fmt.Sprintf("file://%s", dir), dbURL)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			slog.Error("migrate: close", "source_error", srcErr, "database_error", dbErr)
		}
	}()

	switch direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	default:
		return fmt.Errorf("unknown migrate direction %q (want up|down)", direction)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate %s: %w", direction, err)
	}

	slog.Info("migrations applied", "direction", direction)
	return nil
}
