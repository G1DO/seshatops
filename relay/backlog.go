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
func InspectBacklog(ctx context.Context, db *sql.DB) (Backlog, error) {
	var b Backlog
	err := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'publishing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'published' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'quarantined' THEN 1 ELSE 0 END), 0)
		FROM erp.outbox
	`).Scan(&b.Pending, &b.Publishing, &b.Published, &b.Quarantined)
	if err != nil {
		return Backlog{}, fmt.Errorf("relay: backlog counts: %w", err)
	}

	var oldest sql.NullTime
	err = db.QueryRowContext(ctx, `
		SELECT MIN(created_at)
		FROM erp.outbox
		WHERE status IN ('pending', 'publishing')
	`).Scan(&oldest)
	if err != nil {
		return Backlog{}, fmt.Errorf("relay: backlog oldest: %w", err)
	}
	if oldest.Valid {
		b.OldestUnpublished = oldest.Time
	}

	rows, err := db.QueryContext(ctx, `
		SELECT event_id, COALESCE(last_error_code, ''), created_at
		FROM erp.outbox
		WHERE status = 'quarantined'
		ORDER BY created_at
		LIMIT 20
	`)
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
