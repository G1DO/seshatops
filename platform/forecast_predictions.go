package platform

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/G1DO/seshatops/forecast"
)

// PredictionRecord is the Go-owned durable advisory forecast record. It has
// enough immutable lineage to reproduce which input and predictor produced it.
type PredictionRecord struct {
	PredictionID              string
	TenantID                  string
	ResourceType              string
	ResourceID                string
	ObservationDate           string
	ForecastHorizonDays       int
	Target                    string
	Status                    string
	StockoutRisk              *float64
	Uncertainty               *forecast.PredictionUncertainty
	AbstentionReason          string
	EvaluationProtocolVersion string
	DatasetVersion            string
	FeatureDefinitionVersion  string
	FeatureSnapshotID         string
	FeatureSnapshotChecksum   string
	SourceCutoffDate          string
	Predictor                 string
	ModelVersion              string
	CodeVersion               string
	FreshAt                   time.Time
	CorrelationID             string
	RecordedAt                time.Time
}

// PersistPrediction inserts one prediction or returns the existing identical
// record for the same immutable input identity. It never overwrites a result.
func PersistPrediction(ctx context.Context, db *sql.DB, record PredictionRecord) (PredictionRecord, error) {
	if db == nil {
		return PredictionRecord{}, fmt.Errorf("platform: prediction: nil db")
	}
	if err := validatePredictionRecord(&record); err != nil {
		return PredictionRecord{}, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return PredictionRecord{}, fmt.Errorf("platform: prediction begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO platform.forecast_predictions (
			prediction_id, tenant_id, resource_type, resource_id,
			observation_date, forecast_horizon_days, target, status,
			stockout_risk, uncertainty_method, uncertainty_lower,
			uncertainty_upper, uncertainty_sample_count, abstention_reason,
			evaluation_protocol_version, dataset_version,
			feature_definition_version, feature_snapshot_id,
			feature_snapshot_checksum, source_cutoff_date, predictor,
			model_version, code_version, fresh_at, correlation_id, recorded_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26
		) ON CONFLICT DO NOTHING
	`, record.PredictionID, record.TenantID, record.ResourceType, record.ResourceID,
		record.ObservationDate, record.ForecastHorizonDays, record.Target, record.Status,
		record.StockoutRisk, uncertaintyMethod(record.Uncertainty), uncertaintyLower(record.Uncertainty),
		uncertaintyUpper(record.Uncertainty), uncertaintySampleCount(record.Uncertainty), nullableString(record.AbstentionReason),
		record.EvaluationProtocolVersion, record.DatasetVersion, record.FeatureDefinitionVersion,
		record.FeatureSnapshotID, record.FeatureSnapshotChecksum, record.SourceCutoffDate,
		record.Predictor, record.ModelVersion, record.CodeVersion, record.FreshAt,
		record.CorrelationID, record.RecordedAt)
	if err != nil {
		return PredictionRecord{}, fmt.Errorf("platform: prediction insert: %w", err)
	}

	stored, err := scanPrediction(tx.QueryRowContext(ctx, `
		SELECT prediction_id, tenant_id, resource_type, resource_id,
		       observation_date, forecast_horizon_days, target, status,
		       stockout_risk, uncertainty_method, uncertainty_lower,
		       uncertainty_upper, uncertainty_sample_count, abstention_reason,
		       evaluation_protocol_version, dataset_version,
		       feature_definition_version, feature_snapshot_id,
		       feature_snapshot_checksum, source_cutoff_date, predictor,
		       model_version, code_version, fresh_at, correlation_id, recorded_at
		FROM platform.forecast_predictions
		WHERE prediction_id = $1 AND tenant_id = $2
	`, record.PredictionID, record.TenantID))
	if err == sql.ErrNoRows {
		stored, err = scanPrediction(tx.QueryRowContext(ctx, `
			SELECT prediction_id, tenant_id, resource_type, resource_id,
			       observation_date, forecast_horizon_days, target, status,
			       stockout_risk, uncertainty_method, uncertainty_lower,
			       uncertainty_upper, uncertainty_sample_count, abstention_reason,
			       evaluation_protocol_version, dataset_version,
			       feature_definition_version, feature_snapshot_id,
			       feature_snapshot_checksum, source_cutoff_date, predictor,
			       model_version, code_version, fresh_at, correlation_id, recorded_at
			FROM platform.forecast_predictions
			WHERE tenant_id = $1 AND resource_type = $2 AND resource_id = $3
			  AND observation_date = $4 AND forecast_horizon_days = $5
			  AND feature_snapshot_id = $6 AND feature_snapshot_checksum = $7
		`, record.TenantID, record.ResourceType, record.ResourceID, record.ObservationDate,
			record.ForecastHorizonDays, record.FeatureSnapshotID, record.FeatureSnapshotChecksum))
		if err == nil {
			return PredictionRecord{}, fmt.Errorf("%w: immutable prediction identity reused with different id", ErrPredictionConflict)
		}
	}
	if err != nil {
		return PredictionRecord{}, fmt.Errorf("platform: prediction read after insert: %w", err)
	}
	if !samePrediction(record, stored) {
		return PredictionRecord{}, fmt.Errorf("%w: immutable prediction identity reused with different result", ErrPredictionConflict)
	}
	if err := tx.Commit(); err != nil {
		return PredictionRecord{}, fmt.Errorf("platform: prediction commit: %w", err)
	}
	return stored, nil
}

// GetPredictionForTenant reads one prediction only when its tenant matches.
func GetPredictionForTenant(ctx context.Context, db *sql.DB, tenantID, predictionID string) (PredictionRecord, bool, error) {
	if db == nil {
		return PredictionRecord{}, false, fmt.Errorf("platform: prediction: nil db")
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if tenantID == "" || predictionID == "" {
		return PredictionRecord{}, false, ErrInvalidPrediction
	}
	record, err := scanPrediction(db.QueryRowContext(ctx, `
		SELECT prediction_id, tenant_id, resource_type, resource_id,
		       observation_date, forecast_horizon_days, target, status,
		       stockout_risk, uncertainty_method, uncertainty_lower,
		       uncertainty_upper, uncertainty_sample_count, abstention_reason,
		       evaluation_protocol_version, dataset_version,
		       feature_definition_version, feature_snapshot_id,
		       feature_snapshot_checksum, source_cutoff_date, predictor,
		       model_version, code_version, fresh_at, correlation_id, recorded_at
		FROM platform.forecast_predictions
		WHERE tenant_id = $1 AND prediction_id = $2
	`, tenantID, predictionID))
	if err == sql.ErrNoRows {
		return PredictionRecord{}, false, nil
	}
	if err != nil {
		return PredictionRecord{}, false, fmt.Errorf("platform: prediction read: %w", err)
	}
	return record, true, nil
}

// GetLatestPredictionForTenantResource returns the newest immutable prediction
// for one tenant-owned resource. The tenant predicate is part of the query so
// a resource identifier cannot cross the tenant boundary.
func GetLatestPredictionForTenantResource(ctx context.Context, db *sql.DB, tenantID, resourceID string) (PredictionRecord, bool, error) {
	if db == nil {
		return PredictionRecord{}, false, fmt.Errorf("platform: prediction: nil db")
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	resourceID = strings.ToLower(strings.TrimSpace(resourceID))
	if tenantID == "" || resourceID == "" {
		return PredictionRecord{}, false, ErrInvalidPrediction
	}
	record, err := scanPrediction(db.QueryRowContext(ctx, `
		SELECT prediction_id, tenant_id, resource_type, resource_id,
		       observation_date, forecast_horizon_days, target, status,
		       stockout_risk, uncertainty_method, uncertainty_lower,
		       uncertainty_upper, uncertainty_sample_count, abstention_reason,
		       evaluation_protocol_version, dataset_version,
		       feature_definition_version, feature_snapshot_id,
		       feature_snapshot_checksum, source_cutoff_date, predictor,
		       model_version, code_version, fresh_at, correlation_id, recorded_at
		FROM platform.forecast_predictions
		WHERE tenant_id = $1 AND resource_type = 'inventory_item' AND resource_id = $2
		ORDER BY observation_date DESC, recorded_at DESC, prediction_id DESC
		LIMIT 1
	`, tenantID, resourceID))
	if err == sql.ErrNoRows {
		return PredictionRecord{}, false, nil
	}
	if err != nil {
		return PredictionRecord{}, false, fmt.Errorf("platform: prediction latest read: %w", err)
	}
	return record, true, nil
}

// ListPredictionsForTenant returns only same-tenant prediction records.
func ListPredictionsForTenant(ctx context.Context, db *sql.DB, tenantID string) ([]PredictionRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("platform: prediction: nil db")
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if tenantID == "" {
		return nil, ErrInvalidPrediction
	}
	rows, err := db.QueryContext(ctx, `
		SELECT prediction_id, tenant_id, resource_type, resource_id,
		       observation_date, forecast_horizon_days, target, status,
		       stockout_risk, uncertainty_method, uncertainty_lower,
		       uncertainty_upper, uncertainty_sample_count, abstention_reason,
		       evaluation_protocol_version, dataset_version,
		       feature_definition_version, feature_snapshot_id,
		       feature_snapshot_checksum, source_cutoff_date, predictor,
		       model_version, code_version, fresh_at, correlation_id, recorded_at
		FROM platform.forecast_predictions
		WHERE tenant_id = $1
		ORDER BY recorded_at ASC, prediction_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("platform: prediction list: %w", err)
	}
	defer rows.Close()
	var records []PredictionRecord
	for rows.Next() {
		record, err := scanPrediction(rows)
		if err != nil {
			return nil, fmt.Errorf("platform: prediction scan: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform: prediction rows: %w", err)
	}
	return records, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPrediction(row rowScanner) (PredictionRecord, error) {
	var (
		record                             PredictionRecord
		stockoutRisk                       sql.NullFloat64
		uncertaintyMethod                  sql.NullString
		uncertaintyLower, uncertaintyUpper sql.NullFloat64
		uncertaintySampleCount             sql.NullInt64
		abstentionReason                   sql.NullString
	)
	if err := row.Scan(
		&record.PredictionID, &record.TenantID, &record.ResourceType, &record.ResourceID,
		&record.ObservationDate, &record.ForecastHorizonDays, &record.Target, &record.Status,
		&stockoutRisk, &uncertaintyMethod, &uncertaintyLower, &uncertaintyUpper,
		&uncertaintySampleCount, &abstentionReason, &record.EvaluationProtocolVersion,
		&record.DatasetVersion, &record.FeatureDefinitionVersion, &record.FeatureSnapshotID,
		&record.FeatureSnapshotChecksum, &record.SourceCutoffDate, &record.Predictor,
		&record.ModelVersion, &record.CodeVersion, &record.FreshAt, &record.CorrelationID,
		&record.RecordedAt); err != nil {
		return PredictionRecord{}, err
	}
	if stockoutRisk.Valid {
		value := stockoutRisk.Float64
		record.StockoutRisk = &value
	}
	if uncertaintyMethod.Valid {
		uncertainty := forecast.PredictionUncertainty{Method: uncertaintyMethod.String}
		if uncertaintyLower.Valid {
			uncertainty.Lower = uncertaintyLower.Float64
		}
		if uncertaintyUpper.Valid {
			uncertainty.Upper = uncertaintyUpper.Float64
		}
		if uncertaintySampleCount.Valid {
			uncertainty.SampleCount = int(uncertaintySampleCount.Int64)
		}
		record.Uncertainty = &uncertainty
	}
	if abstentionReason.Valid {
		record.AbstentionReason = abstentionReason.String
	}
	record.FreshAt = record.FreshAt.UTC()
	record.RecordedAt = record.RecordedAt.UTC()
	return record, nil
}

func validatePredictionRecord(record *PredictionRecord) error {
	if record.TenantID == "" || record.ResourceType == "" || record.ResourceID == "" || record.Target == "" || record.EvaluationProtocolVersion == "" || record.DatasetVersion == "" || record.FeatureDefinitionVersion == "" || record.FeatureSnapshotID == "" || record.FeatureSnapshotChecksum == "" || record.SourceCutoffDate == "" || record.Predictor == "" || record.ModelVersion == "" || record.CodeVersion == "" || record.CorrelationID == "" {
		return ErrInvalidPrediction
	}
	record.TenantID = strings.ToLower(strings.TrimSpace(record.TenantID))
	record.ResourceID = strings.ToLower(strings.TrimSpace(record.ResourceID))
	record.Target = strings.TrimSpace(record.Target)
	record.CorrelationID = strings.TrimSpace(record.CorrelationID)
	if record.TenantID == "" || record.ResourceID == "" || record.Target != forecast.CandidateTarget || record.ResourceType != "inventory_item" {
		return fmt.Errorf("%w: identity or target", ErrInvalidPrediction)
	}
	if _, err := time.Parse("2006-01-02", record.ObservationDate); err != nil {
		return fmt.Errorf("%w: observation date", ErrInvalidPrediction)
	}
	if _, err := time.Parse("2006-01-02", record.SourceCutoffDate); err != nil || record.SourceCutoffDate != record.ObservationDate {
		return fmt.Errorf("%w: source cutoff", ErrInvalidPrediction)
	}
	if record.ForecastHorizonDays != forecast.HorizonDays || record.Predictor != forecast.RuntimePredictorCandidate && record.Predictor != forecast.RuntimePredictorBaseline {
		return fmt.Errorf("%w: horizon or predictor", ErrInvalidPrediction)
	}
	if record.Status != forecast.CandidatePredictionStatusPredicted && record.Status != forecast.CandidatePredictionStatusAbstained {
		return fmt.Errorf("%w: status", ErrInvalidPrediction)
	}
	if record.Status == forecast.CandidatePredictionStatusPredicted {
		if record.StockoutRisk == nil || record.AbstentionReason != "" || record.Uncertainty == nil {
			return fmt.Errorf("%w: predicted state", ErrInvalidPrediction)
		}
		if math.IsNaN(*record.StockoutRisk) || math.IsInf(*record.StockoutRisk, 0) || *record.StockoutRisk < 0 || *record.StockoutRisk > 1 {
			return fmt.Errorf("%w: score", ErrInvalidPrediction)
		}
	} else if record.StockoutRisk != nil || record.Uncertainty != nil || record.AbstentionReason == "" {
		return fmt.Errorf("%w: abstained state", ErrInvalidPrediction)
	} else if record.AbstentionReason != forecast.CandidateAbstentionInsufficientSupport && record.AbstentionReason != forecast.CandidateAbstentionUnsupportedInput && record.AbstentionReason != forecast.RuntimeAbstentionInsufficientFeatureHistory {
		return fmt.Errorf("%w: abstention reason", ErrInvalidPrediction)
	}
	if record.Uncertainty != nil {
		if record.Uncertainty.Method == "" || math.IsNaN(record.Uncertainty.Lower) || math.IsNaN(record.Uncertainty.Upper) || record.Uncertainty.Lower < 0 || record.Uncertainty.Upper > 1 || record.Uncertainty.Lower > record.Uncertainty.Upper || record.Uncertainty.SampleCount < 0 || (record.Predictor == forecast.RuntimePredictorCandidate && record.Uncertainty.SampleCount == 0) || (record.Predictor == forecast.RuntimePredictorBaseline && record.Uncertainty.Method != forecast.RuntimeUncertaintyDeterministic) {
			return fmt.Errorf("%w: uncertainty", ErrInvalidPrediction)
		}
		if record.StockoutRisk != nil && (*record.StockoutRisk < record.Uncertainty.Lower || *record.StockoutRisk > record.Uncertainty.Upper) {
			return fmt.Errorf("%w: uncertainty bounds", ErrInvalidPrediction)
		}
	}
	if record.RecordedAt.IsZero() {
		record.RecordedAt = time.Now().UTC()
	} else {
		record.RecordedAt = record.RecordedAt.UTC()
	}
	if record.FreshAt.IsZero() {
		record.FreshAt = record.RecordedAt
	} else {
		record.FreshAt = record.FreshAt.UTC()
	}
	wantID := forecast.PredictionIdentity(record.TenantID, record.ResourceID, record.ObservationDate, record.ForecastHorizonDays, record.DatasetVersion, record.FeatureDefinitionVersion, record.FeatureSnapshotID, record.FeatureSnapshotChecksum)
	if record.PredictionID == "" {
		record.PredictionID = wantID
	} else if record.PredictionID != wantID {
		return fmt.Errorf("%w: prediction identity", ErrInvalidPrediction)
	}
	return nil
}

func samePrediction(a, b PredictionRecord) bool {
	return a.PredictionID == b.PredictionID && a.TenantID == b.TenantID && a.ResourceType == b.ResourceType && a.ResourceID == b.ResourceID && a.ObservationDate == b.ObservationDate && a.ForecastHorizonDays == b.ForecastHorizonDays && a.Target == b.Target && a.Status == b.Status && equalFloatPtr(a.StockoutRisk, b.StockoutRisk) && equalUncertainty(a.Uncertainty, b.Uncertainty) && a.AbstentionReason == b.AbstentionReason && a.EvaluationProtocolVersion == b.EvaluationProtocolVersion && a.DatasetVersion == b.DatasetVersion && a.FeatureDefinitionVersion == b.FeatureDefinitionVersion && a.FeatureSnapshotID == b.FeatureSnapshotID && a.FeatureSnapshotChecksum == b.FeatureSnapshotChecksum && a.SourceCutoffDate == b.SourceCutoffDate && a.Predictor == b.Predictor && a.ModelVersion == b.ModelVersion && a.CodeVersion == b.CodeVersion && a.FreshAt.Equal(b.FreshAt)
}

func equalFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalUncertainty(a, b *forecast.PredictionUncertainty) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func uncertaintyMethod(value *forecast.PredictionUncertainty) any {
	if value == nil {
		return nil
	}
	return value.Method
}

func uncertaintyLower(value *forecast.PredictionUncertainty) any {
	if value == nil {
		return nil
	}
	return value.Lower
}

func uncertaintyUpper(value *forecast.PredictionUncertainty) any {
	if value == nil {
		return nil
	}
	return value.Upper
}

func uncertaintySampleCount(value *forecast.PredictionUncertainty) any {
	if value == nil {
		return nil
	}
	return value.SampleCount
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
