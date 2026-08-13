package identity

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	decisionAllow = "allow"
	decisionDeny  = "deny"
)

// DecisionRecord is one append-only privileged authorization decision.
// PrincipalID is the Go-owned session principal, never a client field.
type DecisionRecord struct {
	DecisionID  string
	PrincipalID string
	TenantID    string
	Resource    string
	Action      string
	Outcome     string
	Reason      string
	TargetID    string
	RecordedAt  time.Time
}

// AppendDecision inserts one authorization decision. There is no update or
// delete path. Empty principal, tenant, resource, action, or outcome is
// rejected. Outcome must be allow or deny.
func AppendDecision(ctx context.Context, db *sql.DB, rec DecisionRecord) (DecisionRecord, error) {
	if rec.PrincipalID == "" || rec.TenantID == "" || rec.Resource == "" || rec.Action == "" {
		return DecisionRecord{}, ErrInvalidDecision
	}
	if rec.Outcome != decisionAllow && rec.Outcome != decisionDeny {
		return DecisionRecord{}, ErrInvalidDecision
	}
	if rec.Reason == "" {
		return DecisionRecord{}, ErrInvalidDecision
	}
	if db == nil {
		return DecisionRecord{}, fmt.Errorf("identity: nil db")
	}
	if rec.DecisionID == "" {
		id, err := randomID()
		if err != nil {
			return DecisionRecord{}, fmt.Errorf("identity: decision id: %w", err)
		}
		rec.DecisionID = id
	}
	if rec.RecordedAt.IsZero() {
		rec.RecordedAt = time.Now().UTC()
	} else {
		rec.RecordedAt = rec.RecordedAt.UTC()
	}

	var target any
	if rec.TargetID != "" {
		target = rec.TargetID
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO identity.authorization_decisions (
			decision_id, principal_id, tenant_id, resource, action,
			outcome, reason, target_id, recorded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, rec.DecisionID, rec.PrincipalID, rec.TenantID, rec.Resource, rec.Action,
		rec.Outcome, rec.Reason, target, rec.RecordedAt); err != nil {
		return DecisionRecord{}, fmt.Errorf("identity: append decision: %w", err)
	}
	return rec, nil
}

// ListDecisionsForTenant returns append-only decisions for tenantID ordered
// by recorded_at. Other tenants are never included.
func ListDecisionsForTenant(ctx context.Context, db *sql.DB, tenantID string) ([]DecisionRecord, error) {
	if tenantID == "" {
		return nil, ErrInvalidDecision
	}
	if db == nil {
		return nil, fmt.Errorf("identity: nil db")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT decision_id, principal_id, tenant_id, resource, action,
		       outcome, reason, target_id, recorded_at
		FROM identity.authorization_decisions
		WHERE tenant_id = $1
		ORDER BY recorded_at ASC, decision_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("identity: list decisions: %w", err)
	}
	defer rows.Close()

	out := make([]DecisionRecord, 0)
	for rows.Next() {
		var rec DecisionRecord
		var target sql.NullString
		if err := rows.Scan(
			&rec.DecisionID,
			&rec.PrincipalID,
			&rec.TenantID,
			&rec.Resource,
			&rec.Action,
			&rec.Outcome,
			&rec.Reason,
			&target,
			&rec.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("identity: scan decision: %w", err)
		}
		if target.Valid {
			rec.TargetID = target.String
		}
		rec.RecordedAt = rec.RecordedAt.UTC()
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: list decisions: %w", err)
	}
	return out, nil
}
