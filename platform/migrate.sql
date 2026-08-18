-- Event Spine platform inbox, inventory projection, and lineage projection schema.
-- Owns inbox/deduplication, inventory projection, tenant-scoped lineage
-- projection, and sanitized processing failures.
-- Must not own erp source inventory, lineage source hops, or outbox state.

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE platform.inbox (
    consumer_name TEXT NOT NULL,
    event_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 1),
    disposition TEXT NOT NULL CHECK (disposition IN (
        'applied',
        'duplicate_noop',
        'quarantined_conflict',
        'quarantined_gap',
        'quarantined_stale',
        'quarantined_invalid',
        'quarantined_mismatch',
        'quarantined_transition'
    )),
    expected_version BIGINT,
    received_version BIGINT,
    event_bytes BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer_name, event_id)
);

CREATE INDEX platform_inbox_gap_idx
    ON platform.inbox (consumer_name, tenant_id, aggregate_type, aggregate_id, aggregate_version)
    WHERE disposition = 'quarantined_gap';

CREATE TABLE platform.inventory_projection (
    tenant_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    quantity_on_hand BIGINT NOT NULL CHECK (quantity_on_hand >= 0),
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 0),
    PRIMARY KEY (tenant_id, item_id)
);

-- Relational lineage projection for supplier → lot → batch → shipment → order.
-- No graph store. No cross-table FKs: broker ordering is per aggregate only,
-- so a child hop may arrive before its parent. Traversal always filters by
-- envelope tenant_id; parent ids are never used to infer tenant.
CREATE TABLE platform.lineage_suppliers (
    tenant_id TEXT NOT NULL,
    supplier_id TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 1),
    source_event_id TEXT NOT NULL,
    event_schema_version BIGINT NOT NULL,
    occurred_at TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    causation_id TEXT,
    trace_id TEXT NOT NULL,
    PRIMARY KEY (tenant_id, supplier_id)
);

CREATE TABLE platform.lineage_ingredient_lots (
    tenant_id TEXT NOT NULL,
    lot_id TEXT NOT NULL,
    supplier_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 1),
    source_event_id TEXT NOT NULL,
    event_schema_version BIGINT NOT NULL,
    occurred_at TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    causation_id TEXT,
    trace_id TEXT NOT NULL,
    PRIMARY KEY (tenant_id, lot_id),
    UNIQUE (tenant_id, supplier_id)
);

CREATE TABLE platform.lineage_production_batches (
    tenant_id TEXT NOT NULL,
    batch_id TEXT NOT NULL,
    lot_id TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 1),
    source_event_id TEXT NOT NULL,
    event_schema_version BIGINT NOT NULL,
    occurred_at TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    causation_id TEXT,
    trace_id TEXT NOT NULL,
    PRIMARY KEY (tenant_id, batch_id),
    UNIQUE (tenant_id, lot_id)
);

CREATE TABLE platform.lineage_shipments (
    tenant_id TEXT NOT NULL,
    shipment_id TEXT NOT NULL,
    batch_id TEXT NOT NULL,
    order_id TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 1),
    source_event_id TEXT NOT NULL,
    event_schema_version BIGINT NOT NULL,
    occurred_at TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    causation_id TEXT,
    trace_id TEXT NOT NULL,
    PRIMARY KEY (tenant_id, shipment_id),
    UNIQUE (tenant_id, batch_id),
    UNIQUE (tenant_id, order_id)
);

CREATE TABLE platform.processing_failures (
    failure_id TEXT PRIMARY KEY,
    consumer_name TEXT NOT NULL,
    event_id TEXT,
    tenant_id TEXT,
    aggregate_type TEXT,
    aggregate_id TEXT,
    aggregate_version BIGINT,
    event_schema_version BIGINT,
    event_type TEXT,
    failure_category TEXT NOT NULL,
    content_hash TEXT,
    received_bytes_hash TEXT,
    source_topic TEXT NOT NULL,
    source_partition INTEGER NOT NULL,
    source_offset BIGINT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 1 CHECK (attempt_count >= 1),
    diagnostic_code TEXT NOT NULL,
    quarantine_status TEXT NOT NULL CHECK (quarantine_status IN ('retrying', 'quarantined')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (consumer_name, source_topic, source_partition, source_offset)
);

-- Go-owned advisory forecast results. Python never writes this table.
CREATE TABLE platform.forecast_predictions (
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

CREATE INDEX platform_forecast_predictions_tenant_idx
    ON platform.forecast_predictions (tenant_id, recorded_at, prediction_id);
