package relay

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Status values match erp.outbox CHECK constraints and ADR-0001.
const (
	StatusPending     = "pending"
	StatusPublishing  = "publishing"
	StatusPublished   = "published"
	StatusQuarantined = "quarantined"
)

// Record is one durable outbox row claimed or inspected by the relay.
type Record struct {
	EventID          string
	TenantID         string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	ContentHash      string
	EventBytes       []byte
	Status           string
	RecordedAt       time.Time
	PublishAttempts  int
	LastErrorCode    sql.NullString
	LeaseOwner       sql.NullString
	LeaseExpiresAt   sql.NullTime
	CreatedAt        time.Time
}

// AggregateKey returns the M1 Redpanda partition key:
// tenant_id/aggregate_type/aggregate_id.
func AggregateKey(tenantID, aggregateType, aggregateID string) string {
	return tenantID + "/" + aggregateType + "/" + aggregateID
}

// BackoffAfterAttempts returns the transient-failure delay after n publish
// attempts: 1s exponential growth capped at 60s (CONTRACTS.md §4).
func BackoffAfterAttempts(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	shift := attempts - 1
	if shift > 6 {
		shift = 6
	}
	d := time.Second << shift
	const cap = 60 * time.Second
	if d > cap {
		return cap
	}
	return d
}

// ClaimDue claims up to limit due outbox rows for owner under a lease.
// Due rows are pending with a null or expired publish_lease_expires_at, or
// publishing with an expired lease. Event identity fields are never rewritten.
func ClaimDue(ctx context.Context, db *sql.DB, owner string, leaseTTL time.Duration, limit int) ([]Record, error) {
	if owner == "" {
		return nil, errors.New("relay: claim owner required")
	}
	if leaseTTL <= 0 {
		leaseTTL = DefaultLeaseTTL
	}
	if limit < 1 {
		limit = 1
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("relay: begin claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at, publish_attempts,
			last_error_code, publish_lease_owner, publish_lease_expires_at, created_at
		FROM erp.outbox
		WHERE (
			(status = 'pending' AND (publish_lease_expires_at IS NULL OR publish_lease_expires_at <= now()))
			OR (status = 'publishing' AND (publish_lease_expires_at IS NULL OR publish_lease_expires_at <= now()))
		)
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("relay: select due outbox: %w", err)
	}
	defer rows.Close()

	var claimed []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		claimed = append(claimed, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("relay: iterate due outbox: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("relay: close due outbox: %w", err)
	}

	expiresMs := leaseTTL.Milliseconds()
	out := make([]Record, 0, len(claimed))
	for _, rec := range claimed {
		var (
			newAttempts int
			leaseExp    time.Time
		)
		err := tx.QueryRowContext(ctx, `
			UPDATE erp.outbox
			SET status = 'publishing',
				publish_lease_owner = $2,
				publish_lease_expires_at = now() + ($3 * interval '1 millisecond'),
				publish_attempts = publish_attempts + 1
			WHERE event_id = $1
			RETURNING publish_attempts, publish_lease_expires_at
		`, rec.EventID, owner, expiresMs).Scan(&newAttempts, &leaseExp)
		if err != nil {
			return nil, fmt.Errorf("relay: claim outbox %s: %w", rec.EventID, err)
		}
		rec.Status = StatusPublishing
		rec.PublishAttempts = newAttempts
		rec.LeaseOwner = sql.NullString{String: owner, Valid: true}
		rec.LeaseExpiresAt = sql.NullTime{Time: leaseExp, Valid: true}
		out = append(out, rec)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("relay: commit claim: %w", err)
	}
	return out, nil
}

// MarkPublished sets status published after broker acknowledgement for the
// lease owner. Event identity and bytes are unchanged.
func MarkPublished(ctx context.Context, db *sql.DB, eventID, owner string) error {
	res, err := db.ExecContext(ctx, `
		UPDATE erp.outbox
		SET status = 'published',
			publish_lease_owner = NULL,
			publish_lease_expires_at = NULL,
			last_error_code = NULL
		WHERE event_id = $1
			AND status = 'publishing'
			AND publish_lease_owner = $2
	`, eventID, owner)
	if err != nil {
		return fmt.Errorf("relay: mark published %s: %w", eventID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("relay: mark published rows: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("relay: mark published %s: expected 1 row for owner %q, got %d", eventID, owner, n)
	}
	return nil
}

// ReleaseTransient returns a claimed row to pending with exponential backoff
// stored in publish_lease_expires_at and records last_error_code.
func ReleaseTransient(ctx context.Context, db *sql.DB, eventID, owner, errCode string) error {
	res, err := db.ExecContext(ctx, `
		UPDATE erp.outbox
		SET status = 'pending',
			publish_lease_owner = NULL,
			publish_lease_expires_at = now() + (
				LEAST(60, POWER(2, GREATEST(publish_attempts, 1) - 1)::bigint) * interval '1 second'
			),
			last_error_code = $3
		WHERE event_id = $1
			AND status = 'publishing'
			AND publish_lease_owner = $2
	`, eventID, owner, errCode)
	if err != nil {
		return fmt.Errorf("relay: release transient %s: %w", eventID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("relay: release transient rows: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("relay: release transient %s: expected 1 row, got %d", eventID, n)
	}
	return nil
}

// Quarantine durably marks a non-retryable outbox failure. Rows are never deleted.
func Quarantine(ctx context.Context, db *sql.DB, eventID, owner, errCode string) error {
	res, err := db.ExecContext(ctx, `
		UPDATE erp.outbox
		SET status = 'quarantined',
			publish_lease_owner = NULL,
			publish_lease_expires_at = NULL,
			last_error_code = $3
		WHERE event_id = $1
			AND status = 'publishing'
			AND publish_lease_owner = $2
	`, eventID, owner, errCode)
	if err != nil {
		return fmt.Errorf("relay: quarantine %s: %w", eventID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("relay: quarantine rows: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("relay: quarantine %s: expected 1 row for owner %q, got %d", eventID, owner, n)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(s rowScanner) (Record, error) {
	var rec Record
	err := s.Scan(
		&rec.EventID,
		&rec.TenantID,
		&rec.AggregateType,
		&rec.AggregateID,
		&rec.AggregateVersion,
		&rec.ContentHash,
		&rec.EventBytes,
		&rec.Status,
		&rec.RecordedAt,
		&rec.PublishAttempts,
		&rec.LastErrorCode,
		&rec.LeaseOwner,
		&rec.LeaseExpiresAt,
		&rec.CreatedAt,
	)
	if err != nil {
		return Record{}, fmt.Errorf("relay: scan outbox: %w", err)
	}
	return rec, nil
}
