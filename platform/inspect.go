package platform

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// FailureSample is one sanitized processing_failures row for Event Spine inspection.
type FailureSample struct {
	FailureID        string
	EventID          string
	FailureCategory  string
	DiagnosticCode   string
	QuarantineStatus string
	SourceTopic      string
	SourcePartition  int32
	SourceOffset     int64
	AttemptCount     int
	CreatedAt        time.Time
}

// GapSample is one quarantined_gap inbox row without retained event bytes.
type GapSample struct {
	EventID          string
	TenantID         string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	ExpectedVersion  int64
	ReceivedVersion  int64
	CreatedAt        time.Time
}

// ProcessingInspection is the minimum consumer failure/backlog visibility
// surface for Event Spine verification. It is not an Identity & Operations metrics or alerting stack.
type ProcessingInspection struct {
	Applied               int
	DuplicateNoop         int
	QuarantinedConflict   int
	QuarantinedGap        int
	QuarantinedStale      int
	QuarantinedInvalid    int
	QuarantinedMismatch   int
	QuarantinedTransition int
	FailuresRetrying      int
	FailuresQuarantined   int
	// OldestGap is the earliest created_at among quarantined_gap inbox rows.
	// Zero time means no gap rows.
	OldestGap time.Time
	// OldestFailure is the earliest created_at among processing_failures rows.
	// Zero time means no failure rows.
	OldestFailure time.Time
	Failures      []FailureSample
	Gaps          []GapSample
}

// InspectProcessing returns inbox disposition counts, failure counts, oldest
// gap/failure timestamps, and bounded sanitized samples (LIMIT 20).
func InspectProcessing(ctx context.Context, db *sql.DB) (ProcessingInspection, error) {
	if db == nil {
		return ProcessingInspection{}, fmt.Errorf("%w: nil db", ErrTransient)
	}

	var out ProcessingInspection
	err := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN disposition = 'applied' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN disposition = 'duplicate_noop' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN disposition = 'quarantined_conflict' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN disposition = 'quarantined_gap' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN disposition = 'quarantined_stale' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN disposition = 'quarantined_invalid' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN disposition = 'quarantined_mismatch' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN disposition = 'quarantined_transition' THEN 1 ELSE 0 END), 0)
		FROM platform.inbox
		WHERE consumer_name = $1
	`, ConsumerName).Scan(
		&out.Applied,
		&out.DuplicateNoop,
		&out.QuarantinedConflict,
		&out.QuarantinedGap,
		&out.QuarantinedStale,
		&out.QuarantinedInvalid,
		&out.QuarantinedMismatch,
		&out.QuarantinedTransition,
	)
	if err != nil {
		return ProcessingInspection{}, fmt.Errorf("platform: inspect inbox counts: %w", err)
	}

	err = db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN quarantine_status = 'retrying' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN quarantine_status = 'quarantined' THEN 1 ELSE 0 END), 0)
		FROM platform.processing_failures
		WHERE consumer_name = $1
	`, ConsumerName).Scan(&out.FailuresRetrying, &out.FailuresQuarantined)
	if err != nil {
		return ProcessingInspection{}, fmt.Errorf("platform: inspect failure counts: %w", err)
	}

	var oldestGap sql.NullTime
	err = db.QueryRowContext(ctx, `
		SELECT MIN(created_at)
		FROM platform.inbox
		WHERE consumer_name = $1 AND disposition = 'quarantined_gap'
	`, ConsumerName).Scan(&oldestGap)
	if err != nil {
		return ProcessingInspection{}, fmt.Errorf("platform: inspect oldest gap: %w", err)
	}
	if oldestGap.Valid {
		out.OldestGap = oldestGap.Time
	}

	var oldestFailure sql.NullTime
	err = db.QueryRowContext(ctx, `
		SELECT MIN(created_at)
		FROM platform.processing_failures
		WHERE consumer_name = $1
	`, ConsumerName).Scan(&oldestFailure)
	if err != nil {
		return ProcessingInspection{}, fmt.Errorf("platform: inspect oldest failure: %w", err)
	}
	if oldestFailure.Valid {
		out.OldestFailure = oldestFailure.Time
	}

	failRows, err := db.QueryContext(ctx, `
		SELECT failure_id, COALESCE(event_id, ''), failure_category, diagnostic_code,
		       quarantine_status, source_topic, source_partition, source_offset,
		       attempt_count, created_at
		FROM platform.processing_failures
		WHERE consumer_name = $1
		ORDER BY created_at
		LIMIT 20
	`, ConsumerName)
	if err != nil {
		return ProcessingInspection{}, fmt.Errorf("platform: inspect failure sample: %w", err)
	}
	defer failRows.Close()
	for failRows.Next() {
		var s FailureSample
		if err := failRows.Scan(
			&s.FailureID, &s.EventID, &s.FailureCategory, &s.DiagnosticCode,
			&s.QuarantineStatus, &s.SourceTopic, &s.SourcePartition, &s.SourceOffset,
			&s.AttemptCount, &s.CreatedAt,
		); err != nil {
			return ProcessingInspection{}, fmt.Errorf("platform: scan failure sample: %w", err)
		}
		out.Failures = append(out.Failures, s)
	}
	if err := failRows.Err(); err != nil {
		return ProcessingInspection{}, fmt.Errorf("platform: iterate failure sample: %w", err)
	}

	gapRows, err := db.QueryContext(ctx, `
		SELECT event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
		       COALESCE(expected_version, 0), COALESCE(received_version, 0), created_at
		FROM platform.inbox
		WHERE consumer_name = $1 AND disposition = 'quarantined_gap'
		ORDER BY created_at
		LIMIT 20
	`, ConsumerName)
	if err != nil {
		return ProcessingInspection{}, fmt.Errorf("platform: inspect gap sample: %w", err)
	}
	defer gapRows.Close()
	for gapRows.Next() {
		var s GapSample
		if err := gapRows.Scan(
			&s.EventID, &s.TenantID, &s.AggregateType, &s.AggregateID, &s.AggregateVersion,
			&s.ExpectedVersion, &s.ReceivedVersion, &s.CreatedAt,
		); err != nil {
			return ProcessingInspection{}, fmt.Errorf("platform: scan gap sample: %w", err)
		}
		out.Gaps = append(out.Gaps, s)
	}
	if err := gapRows.Err(); err != nil {
		return ProcessingInspection{}, fmt.Errorf("platform: iterate gap sample: %w", err)
	}
	return out, nil
}
