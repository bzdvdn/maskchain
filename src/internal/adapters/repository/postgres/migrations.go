package postgres

import (
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// @sk-task 30-shield-persistence#T2.4: Run golang-migrate on startup (RQ-001)
func RunMigrations(dsn string) error {
	if dsn == "" {
		return nil
	}

	migrateURL := strings.Replace(dsn, "postgres://", "pgx5://", 1)
	m, err := migrate.New("file://src/internal/adapters/repository/postgres/migrations", migrateURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
