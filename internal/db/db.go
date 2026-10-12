package db

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

var (
	//go:embed migrations/*.sql
	migrationsFS embed.FS
)

func Open(dsn string) (*sql.DB, error) {
	pgConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	connector := stdlib.GetConnector(*pgConfig)

	database := otelsql.OpenDB(connector,
		otelsql.WithAttributes(semconv.DBSystemNamePostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			DisableErrSkip:  true,
			OmitConnPrepare: true,
			OmitRows:        true,
		}),
	)

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(25)
	database.SetConnMaxLifetime(5 * time.Minute)

	return database, nil
}

func Migrate(database *sql.DB) error {
	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(database, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func MigrateDownTo(database *sql.DB, version int64) error {
	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.DownTo(database, "migrations", version); err != nil {
		return fmt.Errorf("failed to roll migrations back: %w", err)
	}

	return nil
}
