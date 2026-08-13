package identity

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrate.sql
var migrateSQL embed.FS

// Migrate applies the identity authorization-decision schema to db.
func Migrate(ctx context.Context, db *sql.DB) error {
	raw, err := migrateSQL.ReadFile("migrate.sql")
	if err != nil {
		return fmt.Errorf("identity: read migrate.sql: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(raw)); err != nil {
		return fmt.Errorf("identity: migrate: %w", err)
	}
	return nil
}
