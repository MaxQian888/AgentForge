package database

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	log "github.com/sirupsen/logrus"
)

// RunMigrations runs all pending migrations from an embedded filesystem.
func RunMigrations(postgresURL string, migrations fs.FS) error {
	log.Info("starting database migrations")

	source, err := iofs.New(migrations, ".")
	if err != nil {
		return fmt.Errorf("create iofs source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, postgresURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.WithError(srcErr).Warn("close migration source")
		}
		if dbErr != nil {
			log.WithError(dbErr).Warn("close migration db")
		}
	}()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("database migrations already up to date")
			return nil
		}
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Info("database migrations applied")
	return nil
}
