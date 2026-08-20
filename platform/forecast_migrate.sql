-- Idempotent schema needed by the one-shot forecast command.
CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.forecast_predictions (
    prediction_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    observation_date TEXT NOT NULL,
    forecast_horizon_days INTEGER NOT NULL CHECK (forecast_horizon_days > 0),
    target TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('predicted', 'abstained')),
    stockout_risk DOUBLE PRECISION,
    uncertainty_method TEXT,
    uncertainty_lower DOUBLE PRECISION,
    uncertainty_upper DOUBLE PRECISION,
    uncertainty_sample_count INTEGER CHECK (uncertainty_sample_count >= 0),
    abstention_reason TEXT,
    evaluation_protocol_version TEXT NOT NULL,
    dataset_version TEXT NOT NULL,
    feature_definition_version TEXT NOT NULL,
    feature_snapshot_id TEXT NOT NULL,
    feature_snapshot_checksum TEXT NOT NULL,
    source_cutoff_date TEXT NOT NULL,
    predictor TEXT NOT NULL CHECK (predictor IN ('candidate', 'baseline')),
    model_version TEXT NOT NULL,
    code_version TEXT NOT NULL,
    fresh_at TIMESTAMPTZ NOT NULL,
    correlation_id TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    UNIQUE (
        tenant_id, resource_type, resource_id, observation_date,
        forecast_horizon_days, feature_snapshot_id, feature_snapshot_checksum
    )
);

CREATE INDEX IF NOT EXISTS platform_forecast_predictions_tenant_idx
    ON platform.forecast_predictions (tenant_id, recorded_at, prediction_id);
