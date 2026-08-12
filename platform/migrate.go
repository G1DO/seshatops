package platform

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrate.sql
var migrateSQL embed.FS

// Migrate applies the M1 platform schema to db.
func Migrate(ctx context.Context, db *sql.DB) error {
	raw, err := migrateSQL.ReadFile("migrate.sql")
	if err != nil {
		return fmt.Errorf("platform: read migrate.sql: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(raw)); err != nil {
		return fmt.Errorf("platform: migrate: %w", err)
	}
	return nil
}
