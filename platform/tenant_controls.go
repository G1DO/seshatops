package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/relay"
)

// OperatorReplayPartition is the synthetic partition used for operator-initiated
// ProcessRecord calls so they do not collide with consumer (topic, partition,
// offset) uniqueness on processing_failures.
const OperatorReplayPartition int32 = 1

// ResetDerivedStateForTenant deletes derived platform state for one tenant:
// inbox, inventory_projection, and processing_failures rows with that tenant_id.
// Unattributed processing_failures (tenant_id NULL) and other tenants are
// untouched. It never mutates the erp schema or broker history.
func ResetDerivedStateForTenant(ctx context.Context, db *sql.DB, tenantID string) error {
	if db == nil {
		return fmt.Errorf("platform: nil db")
	}
	if tenantID == "" {
		return fmt.Errorf("platform: tenant_id required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("platform: begin tenant reset: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM platform.processing_failures WHERE tenant_id = $1
	`, tenantID); err != nil {
		return fmt.Errorf("platform: reset tenant failures: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM platform.inbox WHERE tenant_id = $1
	`, tenantID); err != nil {
		return fmt.Errorf("platform: reset tenant inbox: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM platform.inventory_projection WHERE tenant_id = $1
	`, tenantID); err != nil {
		return fmt.Errorf("platform: reset tenant projection: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("platform: commit tenant reset: %w", err)
	}
	return nil
}

// LoadTenantOutboxHistory loads retained erp.outbox bytes for tenantID.
// When eventID is non-empty, only that row is returned. Envelope tenant_id
// must match tenantID; a mismatch is ErrTenantMismatch and no records apply.
func LoadTenantOutboxHistory(ctx context.Context, db *sql.DB, tenantID, eventID string) ([]HistoryRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("platform: nil db")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("platform: tenant_id required")
	}

	query := `
		SELECT event_id, tenant_id, aggregate_type, aggregate_id, event_bytes
		FROM erp.outbox
		WHERE tenant_id = $1
	`
	args := []any{tenantID}
	if eventID != "" {
		query += ` AND event_id = $2`
		args = append(args, eventID)
	}
	query += ` ORDER BY aggregate_id, aggregate_version, recorded_at, event_id`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("platform: load tenant outbox: %w", err)
	}
	defer rows.Close()

	var out []HistoryRecord
	var offset int64
	for rows.Next() {
		var (
			rowEventID    string
			rowTenantID   string
			aggregateType string
			aggregateID   string
			value         []byte
		)
		if err := rows.Scan(&rowEventID, &rowTenantID, &aggregateType, &aggregateID, &value); err != nil {
			return nil, fmt.Errorf("platform: scan tenant outbox: %w", err)
		}
		if rowTenantID != tenantID {
			return nil, ErrTenantMismatch
		}
		env, err := event.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("platform: parse retained outbox %s: %w", rowEventID, err)
		}
		if env.TenantID != tenantID {
			return nil, ErrTenantMismatch
		}
		offset++
		out = append(out, HistoryRecord{
			Key:   []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID)),
			Value: value,
			Pos: SourcePosition{
				Topic:     relay.Topic,
				Partition: OperatorReplayPartition,
				Offset:    offset,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform: iterate tenant outbox: %w", err)
	}
	return out, nil
}

// ReplayTenantHistory reprocesses retained same-tenant outbox bytes through
// ProcessRecord without resetting derived state. Already-applied events are
// duplicate_noop. eventID, when set, selects one row.
func ReplayTenantHistory(ctx context.Context, db *sql.DB, tenantID, eventID string) (RebuildResult, error) {
	records, err := LoadTenantOutboxHistory(ctx, db, tenantID, eventID)
	if err != nil {
		return RebuildResult{}, err
	}
	if eventID != "" && len(records) == 0 {
		return RebuildResult{}, ErrControlNotFound
	}
	return replayTenantRecords(ctx, db, tenantID, records)
}

// RebuildTenantFromHistory resets derived state for tenantID only, then
// replays that tenant's retained outbox bytes. Incomplete status must not be
// treated as a successful checksum proof. Other tenants are untouched.
func RebuildTenantFromHistory(ctx context.Context, db *sql.DB, tenantID string) (RebuildResult, error) {
	if err := ResetDerivedStateForTenant(ctx, db, tenantID); err != nil {
		return RebuildResult{}, err
	}
	records, err := LoadTenantOutboxHistory(ctx, db, tenantID, "")
	if err != nil {
		return RebuildResult{}, err
	}
	return replayTenantRecords(ctx, db, tenantID, records)
}

// ReleaseTenantQuarantine retries a same-tenant quarantined outbox row or
// re-drives a same-tenant poison failure through ProcessRecord. Terminal inbox
// quarantines are never force-applied.
func ReleaseTenantQuarantine(ctx context.Context, db *sql.DB, tenantID, eventID string) error {
	if db == nil {
		return fmt.Errorf("platform: nil db")
	}
	if tenantID == "" || eventID == "" {
		return ErrControlNotFound
	}

	disp, found, err := inboxDispositionForTenant(ctx, db, tenantID, eventID)
	if err != nil {
		return err
	}
	if found && isTerminalInboxQuarantine(disp) {
		return ErrNotReleasable
	}

	err = relay.ReleaseQuarantined(ctx, db, tenantID, eventID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, relay.ErrQuarantineNotFound) {
		return err
	}

	hasFail, err := hasQuarantinedFailure(ctx, db, tenantID, eventID)
	if err != nil {
		return err
	}
	if !hasFail {
		return ErrControlNotFound
	}
	records, err := LoadTenantOutboxHistory(ctx, db, tenantID, eventID)
	if err != nil {
		return err
	}
	if len(records) != 1 {
		return ErrNotReleasable
	}
	_, err = ProcessRecord(ctx, db, records[0].Key, records[0].Value, records[0].Pos)
	return err
}

func inboxDispositionForTenant(ctx context.Context, db *sql.DB, tenantID, eventID string) (string, bool, error) {
	var disp string
	err := db.QueryRowContext(ctx, `
		SELECT disposition FROM platform.inbox
		WHERE consumer_name = $1 AND event_id = $2 AND tenant_id = $3
	`, ConsumerName, eventID, tenantID).Scan(&disp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("platform: inbox disposition: %w", err)
	}
	return disp, true, nil
}

func hasQuarantinedFailure(ctx context.Context, db *sql.DB, tenantID, eventID string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM platform.processing_failures
		WHERE consumer_name = $1
		  AND event_id = $2
		  AND tenant_id = $3
		  AND quarantine_status = 'quarantined'
	`, ConsumerName, eventID, tenantID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("platform: count quarantined failures: %w", err)
	}
	return n > 0, nil
}

func isTerminalInboxQuarantine(disp string) bool {
	switch disp {
	case DispositionQuarantinedGap,
		DispositionQuarantinedConflict,
		DispositionQuarantinedStale,
		DispositionQuarantinedInvalid,
		DispositionQuarantinedMismatch,
		DispositionQuarantinedTransition:
		return true
	default:
		return false
	}
}

func replayTenantRecords(ctx context.Context, db *sql.DB, tenantID string, records []HistoryRecord) (RebuildResult, error) {
	out := RebuildResult{
		Status: RebuildStatusComplete,
		Metadata: ReproductionMetadata{
			HandlerVersion:       HandlerVersion,
			EventContractVersion: event.SchemaVersionV1,
			BrokerTopic:          relay.Topic,
			ChecksumInputs:       "tenant inventory_projection rows: tenant_id, item_id, quantity_on_hand, aggregate_version (CONTRACTS.md §8)",
			Limitations:          "Tenant-scoped operator replay/rebuild; not Traceability & Recovery backup/restore; not a hosted FC-014 campaign.",
		},
	}
	if len(records) == 0 {
		out.Status = RebuildStatusIncomplete
		out.IncompleteReasons = append(out.IncompleteReasons, "empty retained history")
		out.Metadata.RebuildStatus = out.Status
		return out, nil
	}

	out.Metadata.BrokerPartitionMin = records[0].Pos.Partition
	out.Metadata.BrokerPartitionMax = records[0].Pos.Partition
	out.Metadata.BrokerOffsetMin = records[0].Pos.Offset
	out.Metadata.BrokerOffsetMax = records[0].Pos.Offset

	for i, rec := range records {
		pos := rec.Pos
		if pos.Topic == "" {
			pos.Topic = relay.Topic
		}
		if pos.Partition == 0 {
			pos.Partition = OperatorReplayPartition
		}
		if pos.Partition < out.Metadata.BrokerPartitionMin {
			out.Metadata.BrokerPartitionMin = pos.Partition
		}
		if pos.Partition > out.Metadata.BrokerPartitionMax {
			out.Metadata.BrokerPartitionMax = pos.Partition
		}
		if pos.Offset < out.Metadata.BrokerOffsetMin {
			out.Metadata.BrokerOffsetMin = pos.Offset
		}
		if pos.Offset > out.Metadata.BrokerOffsetMax {
			out.Metadata.BrokerOffsetMax = pos.Offset
		}

		env, err := event.Parse(rec.Value)
		if err != nil {
			out.Status = RebuildStatusIncomplete
			out.IncompleteReasons = append(out.IncompleteReasons,
				fmt.Sprintf("record[%d] parse: %v", i, err))
			out.Metadata.RebuildStatus = out.Status
			return out, nil
		}
		if env.TenantID != tenantID {
			return RebuildResult{}, ErrTenantMismatch
		}

		res, err := ProcessRecord(ctx, db, rec.Key, rec.Value, pos)
		if err != nil && !res.ShouldAck {
			out.Status = RebuildStatusIncomplete
			out.IncompleteReasons = append(out.IncompleteReasons,
				fmt.Sprintf("record[%d] transient/non-ack failure: %v", i, err))
			out.Metadata.RebuildStatus = out.Status
			return out, err
		}
		disp := res.Disposition
		if disp == "" {
			disp = "(none)"
		}
		out.Dispositions = append(out.Dispositions, disp)

		switch res.Disposition {
		case DispositionApplied:
			out.Applied++
		case DispositionDuplicateNoop:
			out.DuplicateNoop++
		case DispositionQuarantinedGap:
			out.Quarantined++
		case DispositionQuarantinedConflict,
			DispositionQuarantinedStale,
			DispositionQuarantinedInvalid,
			DispositionQuarantinedMismatch,
			DispositionQuarantinedTransition:
			out.Quarantined++
			out.Status = RebuildStatusIncomplete
			out.IncompleteReasons = append(out.IncompleteReasons,
				fmt.Sprintf("record[%d] disposition %s", i, res.Disposition))
		default:
			if res.Disposition == "" {
				out.Failures++
				out.Status = RebuildStatusIncomplete
				out.IncompleteReasons = append(out.IncompleteReasons,
					fmt.Sprintf("record[%d] missing disposition (failure path)", i))
			} else {
				out.Quarantined++
				out.Status = RebuildStatusIncomplete
				out.IncompleteReasons = append(out.IncompleteReasons,
					fmt.Sprintf("record[%d] unexpected disposition %s", i, res.Disposition))
			}
		}
	}

	var gapCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM platform.inbox
		WHERE consumer_name = $1 AND tenant_id = $2 AND disposition = $3
	`, ConsumerName, tenantID, DispositionQuarantinedGap).Scan(&gapCount); err != nil {
		return out, fmt.Errorf("platform: count tenant residual gaps: %w", err)
	}
	if gapCount > 0 {
		out.Status = RebuildStatusIncomplete
		out.IncompleteReasons = append(out.IncompleteReasons,
			fmt.Sprintf("%d residual quarantined_gap row(s)", gapCount))
	}

	var failureCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM platform.processing_failures
		WHERE consumer_name = $1 AND tenant_id = $2
	`, ConsumerName, tenantID).Scan(&failureCount); err != nil {
		return out, fmt.Errorf("platform: count tenant processing failures: %w", err)
	}
	if failureCount > 0 {
		out.Failures = failureCount
		out.Status = RebuildStatusIncomplete
		out.IncompleteReasons = append(out.IncompleteReasons,
			fmt.Sprintf("%d processing_failures row(s)", failureCount))
	}

	sum, err := ChecksumTenant(ctx, db, tenantID)
	if err != nil {
		return out, err
	}
	out.Checksum = sum
	out.Metadata.Checksum = sum
	out.Metadata.RebuildStatus = out.Status
	return out, nil
}
