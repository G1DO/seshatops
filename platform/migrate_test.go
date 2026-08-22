package platform

import (
	"context"
	"database/sql"
	"testing"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/identity"
)

func TestForecastMigrationIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := MigrateForecast(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := MigrateForecast(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeMigrationsAreIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	for name, migrate := range map[string]func(context.Context, *sql.DB) error{
		"erp":      erp.Migrate,
		"platform": Migrate,
		"identity": identity.Migrate,
	} {
		if err := migrate(ctx, db); err != nil {
			t.Fatalf("first %s migration: %v", name, err)
		}
		if err := migrate(ctx, db); err != nil {
			t.Fatalf("second %s migration: %v", name, err)
		}
	}
}
