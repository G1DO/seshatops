package platform

import (
	"context"
	"testing"
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
