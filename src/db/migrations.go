package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func runMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create database driver: %v", err)
	}

	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("could not create iofs source: %v", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %v", err)
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("could not run up migrations: %v", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("No migrations to apply")
	} else {
		log.Println("Migrations applied successfully")
	}

	return nil
}
