package main

import (
	"errors"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/react-go-quick-starter/server/internal/config"
	migrationsfs "github.com/react-go-quick-starter/server/migrations"
)

func main() {
	cfg := config.Load()
	if cfg.PostgresURL == "" {
		log.Fatal("POSTGRES_URL is required")
	}

	source, err := iofs.New(migrationsfs.FS, ".")
	if err != nil {
		log.Printf("migration source init failed: %v", err)
		os.Exit(1)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, cfg.PostgresURL)
	if err != nil {
		log.Printf("migration init failed: %v", err)
		os.Exit(1)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("close migration source failed: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("close migration db failed: %v", dbErr)
		}
	}()

	if version, dirty, versionErr := m.Version(); versionErr == nil && dirty {
		log.Printf("forcing dirty migration version %d clean", version)
		if forceErr := m.Force(int(version)); forceErr != nil {
			log.Printf("force migration version failed: %v", forceErr)
			os.Exit(1)
		}
	}

	log.Print("starting database migrations")
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Printf("migration run failed: %v", err)
		os.Exit(1)
	}

	log.Println("migration run complete")
}
