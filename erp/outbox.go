package erp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/G1DO/seshatops/event"
)

// testFailBeforeCommit, when set by same-package tests, runs after all writes
// and before Commit. A non-nil error forces Rollback.
var testFailBeforeCommit func(ctx context.Context) error

func setTestFailBeforeCommitForTest(fn func(ctx context.Context) error) {
	testFailBeforeCommit = fn
}

func insertPendingOutbox(ctx context.Context, tx *sql.Tx, env event.Envelope, recordedAt time.Time) ([]byte, string, error) {
	if err := event.Validate(env); err != nil {
		return nil, "", err
	}
	eventBytes, err := event.CanonicalBytes(env)
	if err != nil {
		return nil, "", fmt.Errorf("erp: canonical bytes: %w", err)
	}
	contentHash, err := event.ContentHash(env)
	if err != nil {
		return nil, "", fmt.Errorf("erp: content hash: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO erp.outbox (
			event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8)
	`, env.EventID, env.TenantID, env.AggregateType, env.AggregateID, env.AggregateVersion,
		contentHash, eventBytes, recordedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, "", ErrDuplicateEvent
		}
		return nil, "", fmt.Errorf("erp: insert outbox: %w", err)
	}
	return eventBytes, contentHash, nil
}

func commitSource(ctx context.Context, tx *sql.Tx) error {
	if testFailBeforeCommit != nil {
		if err := testFailBeforeCommit(ctx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erp: commit: %w", err)
	}
	return nil
}

func parentOutboxEventID(ctx context.Context, tx *sql.Tx, tenantID, aggregateType, aggregateID string) (string, error) {
	var eventID string
	err := tx.QueryRowContext(ctx, `
		SELECT event_id
		FROM erp.outbox
		WHERE tenant_id = $1 AND aggregate_type = $2 AND aggregate_id = $3
		ORDER BY aggregate_version
		LIMIT 1
	`, tenantID, aggregateType, aggregateID).Scan(&eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnknownSource
	}
	if err != nil {
		return "", fmt.Errorf("erp: parent outbox: %w", err)
	}
	return eventID, nil
}

func requireCausation(got *string, want string) error {
	if got == nil || *got != want {
		return ErrInvalidTransition
	}
	return nil
}
