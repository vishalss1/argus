package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lib/pq"
)

type scanner interface {
	Scan(dest ...any) error
}

func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23503"
}

func InitDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Retry connection with backoff — postgres may not be ready yet
	var pingErr error
	for attempt := 1; attempt <= 10; attempt++ {
		pingErr = db.Ping()
		if pingErr == nil {
			break
		}
		log.Printf("Database ping attempt %d/10 failed: %v", attempt, pingErr)
		if attempt < 10 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	if pingErr != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database after 10 attempts: %v", pingErr)
	}

	log.Println("successfully connected to the database")

	err = runMigrations(db)
	if err != nil {
		log.Printf("Warning: Migrations failed: %v", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create database driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
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


