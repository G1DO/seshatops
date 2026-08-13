-- Identity authorization-decision audit schema (Issue #49).
-- Owns append-only privileged allow/deny records. Must not own erp source
-- inventory, outbox, or platform projection state.

CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE IF NOT EXISTS identity.authorization_decisions (
    decision_id TEXT PRIMARY KEY,
    principal_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('allow', 'deny')),
    reason TEXT NOT NULL,
    target_id TEXT,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS authorization_decisions_tenant_recorded_idx
    ON identity.authorization_decisions (tenant_id, recorded_at, decision_id);

CREATE OR REPLACE FUNCTION identity.reject_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'identity.authorization_decisions is append-only';
END;
$$;

DROP TRIGGER IF EXISTS authorization_decisions_no_update ON identity.authorization_decisions;
CREATE TRIGGER authorization_decisions_no_update
    BEFORE UPDATE OR DELETE ON identity.authorization_decisions
    FOR EACH ROW
    EXECUTE FUNCTION identity.reject_audit_mutation();
