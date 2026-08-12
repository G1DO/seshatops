-- M1 platform inbox and inventory projection schema (Issue #25).
-- Owns inbox/deduplication, inventory projection, and sanitized processing failures.
-- Must not own erp source inventory or outbox state.

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
