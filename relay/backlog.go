package relay

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// QuarantineSample is one quarantined outbox row for backlog inspection.
type QuarantineSample struct {
	EventID       string
	LastErrorCode string
	CreatedAt     time.Time
}

// Backlog is the minimum unpublished/failure visibility surface for Event Spine.
type Backlog struct {
	Pending     int
	Publishing  int
	Published   int
	Quarantined int
	// OldestUnpublished is the earliest created_at among pending or publishing
	// rows. Zero time means no unpublished rows.
	OldestUnpublished time.Time
	Quarantines       []QuarantineSample
}

// InspectBacklog returns counts and a bounded quarantine sample for verification.
// This is the Event Spine library surface (all tenants). Issue #47 product
// reads use InspectBacklogForTenant.
func InspectBacklog(ctx context.Context, db *sql.DB) (Backlog, error) {
	return inspectBacklog(ctx, db, "")
}

// InspectBacklogForTenant returns the same backlog signals as InspectBacklog,
// scoped to one tenant. Empty tenantID fails closed.
func InspectBacklogForTenant(ctx context.Context, db *sql.DB, tenantID string) (Backlog, error) {
	if tenantID == "" {
		return Backlog{}, fmt.Errorf("relay: inspect backlog: empty tenant_id")
	}
	return inspectBacklog(ctx, db, tenantID)
}

func inspectBacklog(ctx context.Context, db *sql.DB, tenantID string) (Backlog, error) {
	if db == nil {
		return Backlog{}, fmt.Errorf("relay: inspect backlog: nil db")
	}

	tenantPred, args := tenantPredicate(tenantID, 1)
	countSQL := `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'publishing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'published' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'quarantined' THEN 1 ELSE 0 END), 0)
		FROM erp.outbox` + tenantPred

	var b Backlog
	err := db.QueryRowContext(ctx, countSQL, args...).Scan(&b.Pending, &b.Publishing, &b.Published, &b.Quarantined)
	if err != nil {
		return Backlog{}, fmt.Errorf("relay: backlog counts: %w", err)
	}

	oldestPred, oldestArgs := tenantAndStatus(tenantID, "status IN ('pending', 'publishing')")
	var oldest sql.NullTime
	err = db.QueryRowContext(ctx, `
		SELECT MIN(created_at)
		FROM erp.outbox
	`+oldestPred, oldestArgs...).Scan(&oldest)
	if err != nil {
		return Backlog{}, fmt.Errorf("relay: backlog oldest: %w", err)
	}
	if oldest.Valid {
		b.OldestUnpublished = oldest.Time
	}

	samplePred, sampleArgs := tenantAndStatus(tenantID, "status = 'quarantined'")
	rows, err := db.QueryContext(ctx, `
		SELECT event_id, COALESCE(last_error_code, ''), created_at
		FROM erp.outbox
	`+samplePred+`
		ORDER BY created_at
		LIMIT 20
	`, sampleArgs...)
	if err != nil {
		return Backlog{}, fmt.Errorf("relay: backlog quarantine sample: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var q QuarantineSample
		if err := rows.Scan(&q.EventID, &q.LastErrorCode, &q.CreatedAt); err != nil {
			return Backlog{}, fmt.Errorf("relay: scan quarantine sample: %w", err)
		}
		b.Quarantines = append(b.Quarantines, q)
	}
	if err := rows.Err(); err != nil {
		return Backlog{}, fmt.Errorf("relay: iterate quarantine sample: %w", err)
	}
	return b, nil
}

func tenantPredicate(tenantID string, argPos int) (string, []any) {
	if tenantID == "" {
		return "", nil
	}
	return fmt.Sprintf(" WHERE tenant_id = $%d", argPos), []any{tenantID}
}

func tenantAndStatus(tenantID, statusPred string) (string, []any) {
	if tenantID == "" {
		return " WHERE " + statusPred, nil
	}
	return " WHERE tenant_id = $1 AND " + statusPred, []any{tenantID}
}
