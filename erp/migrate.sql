-- M1 synthetic ERP source schema (Issue #23).
-- Owns orders, authoritative inventory, and outbox intent only.

CREATE SCHEMA IF NOT EXISTS erp;

CREATE TABLE erp.inventory_items (
    tenant_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    quantity_on_hand BIGINT NOT NULL CHECK (quantity_on_hand >= 0),
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 0),
    PRIMARY KEY (tenant_id, item_id)
);

CREATE TABLE erp.orders (
    order_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    quantity BIGINT NOT NULL CHECK (quantity >= 1),
    occurred_at TIMESTAMPTZ NOT NULL,
    correlation_id TEXT NOT NULL,
    trace_id TEXT NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE erp.outbox (
    event_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 1),
    content_hash TEXT NOT NULL,
    event_bytes BYTEA NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'publishing', 'published', 'quarantined')),
    recorded_at TIMESTAMPTZ NOT NULL,
    publish_lease_owner TEXT,
    publish_lease_expires_at TIMESTAMPTZ,
    publish_attempts INTEGER NOT NULL DEFAULT 0,
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX erp_outbox_pending_idx
    ON erp.outbox (status, created_at)
    WHERE status IN ('pending', 'publishing');
