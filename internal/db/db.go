package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn())
}

func Migrate(pool *pgxpool.Pool) error {
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	// golang-migrate works best with database/sql — we use pgx's stdlib adapter
	// which registers itself as the "pgx" driver name
	db, err := sql.Open("pgx", dsn())
	if err != nil {
		return fmt.Errorf("migrate db open: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// dsn builds the connection string used by both pgxpool and migrate
func dsn() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=prefer",
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_USER", "bunkum_api"),
		os.Getenv("POSTGRES_PASSWORD"),
		getenv("DB_NAME", "bunkum_api_production"),
	)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
