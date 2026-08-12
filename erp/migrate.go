package erp

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrate.sql
var migrateSQL embed.FS

// PostgresImage is the immutable PostgreSQL 16.14 image pin for Event Spine local and
// integration tooling (CONTRACTS.md §9).
const PostgresImage = "postgres@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b"

// PostgresVersionLabel records the human-readable version paired with PostgresImage.
const PostgresVersionLabel = "16.14"

// Migrate applies the Event Spine erp schema to db.
func Migrate(ctx context.Context, db *sql.DB) error {
	raw, err := migrateSQL.ReadFile("migrate.sql")
	if err != nil {
		return fmt.Errorf("erp: read migrate.sql: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(raw)); err != nil {
		return fmt.Errorf("erp: migrate: %w", err)
	}
	return nil
}
