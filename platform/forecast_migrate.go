package platform

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed forecast_migrate.sql
var forecastMigrateSQL embed.FS

// MigrateForecast applies the idempotent forecast persistence schema used by
// the one-shot forecast command and rejects an incompatible existing table.
func MigrateForecast(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("platform: forecast migrate: nil db")
	}
	raw, err := forecastMigrateSQL.ReadFile("forecast_migrate.sql")
	if err != nil {
		return fmt.Errorf("platform: read forecast migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(raw)); err != nil {
		return fmt.Errorf("platform: forecast migrate: %w", err)
	}
	if err := validateForecastPredictionSchema(ctx, db); err != nil {
		return err
	}
	return nil
}

var forecastPredictionColumns = []string{
	"prediction_id", "tenant_id", "resource_type", "resource_id",
	"observation_date", "forecast_horizon_days", "target", "status",
	"stockout_risk", "uncertainty_method", "uncertainty_lower",
	"uncertainty_upper", "uncertainty_sample_count", "abstention_reason",
	"evaluation_protocol_version", "dataset_version",
	"feature_definition_version", "feature_snapshot_id",
	"feature_snapshot_checksum", "source_cutoff_date", "predictor",
	"model_version", "code_version", "fresh_at", "correlation_id", "recorded_at",
}

func validateForecastPredictionSchema(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'platform' AND table_name = 'forecast_predictions'
	`)
	if err != nil {
		return fmt.Errorf("platform: forecast schema columns: %w", err)
	}
	columns := make(map[string]struct{}, len(forecastPredictionColumns))
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			_ = rows.Close()
			return fmt.Errorf("platform: forecast schema column scan: %w", err)
		}
		columns[column] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("platform: forecast schema columns: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("platform: forecast schema columns close: %w", err)
	}
	for _, column := range forecastPredictionColumns {
		if _, ok := columns[column]; !ok {
			return fmt.Errorf("platform: forecast schema missing column %s", column)
		}
	}

	constraintRows, err := db.QueryContext(ctx, `
		SELECT contype, pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = 'platform.forecast_predictions'::regclass
	`)
	if err != nil {
		return fmt.Errorf("platform: forecast schema constraints: %w", err)
	}
	hasPrimaryKey := false
	hasIdentityUnique := false
	for constraintRows.Next() {
		var kind, definition string
		if err := constraintRows.Scan(&kind, &definition); err != nil {
			_ = constraintRows.Close()
			return fmt.Errorf("platform: forecast schema constraint scan: %w", err)
		}
		definition = strings.ToLower(strings.Join(strings.Fields(definition), " "))
		if kind == "p" && strings.Contains(definition, "primary key (prediction_id)") {
			hasPrimaryKey = true
		}
		if kind == "u" && strings.Contains(definition, "unique (tenant_id, resource_type, resource_id, observation_date, forecast_horizon_days, feature_snapshot_id, feature_snapshot_checksum)") {
			hasIdentityUnique = true
		}
	}
	if err := constraintRows.Err(); err != nil {
		_ = constraintRows.Close()
		return fmt.Errorf("platform: forecast schema constraints: %w", err)
	}
	if err := constraintRows.Close(); err != nil {
		return fmt.Errorf("platform: forecast schema constraints close: %w", err)
	}
	if !hasPrimaryKey || !hasIdentityUnique {
		return fmt.Errorf("platform: forecast schema missing immutable prediction constraints")
	}
	return nil
}
