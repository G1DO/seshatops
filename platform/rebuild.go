package platform

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/relay"
)

// HandlerVersion identifies the M1 inventory-projection handler logic used for
// reproduction metadata (EVENT_MODEL.md §7). It is distinct from
// event_schema_version.
const HandlerVersion = "m1-inventory-projection-v1"

// Rebuild status values.
const (
	RebuildStatusComplete   = "complete"
	RebuildStatusIncomplete = "incomplete"
)

// HistoryRecord is one retained broker (or captured) delivery used as replay
// input. Value must be the exact retained event bytes.
type HistoryRecord struct {
	Key   []byte
	Value []byte
	Pos   SourcePosition
}

// RebuildOptions controls RebuildFromHistory.
type RebuildOptions struct {
	// TenantID selects the tenant for ChecksumTenant after replay.
	TenantID string
	// ResetFirst, when true, calls ResetDerivedState before processing.
	ResetFirst bool
	// Metadata supplies reproduction fields; HandlerVersion and contract
	// version are filled by RebuildFromHistory when empty.
	Metadata ReproductionMetadata
}

// ReproductionMetadata records the #29 reproduction context for a rebuild or
// duplicate-safety campaign. Commit is caller-supplied (often filled in the
// review record); tests may leave it empty.
type ReproductionMetadata struct {
	Seed                 string
	EventContractVersion int64
	HandlerVersion       string
	Commit               string
	BrokerTopic          string
	BrokerPartitionMin   int32
	BrokerPartitionMax   int32
	BrokerOffsetMin      int64
	BrokerOffsetMax      int64
	ChecksumInputs       string
	Checksum             string
	RebuildStatus        string
	Limitations          string
}

// RebuildResult is the outcome of replaying retained history through the
// projection handler. Status incomplete means the checksum must not be treated
// as a successful A==B proof.
type RebuildResult struct {
	Status            string
	Checksum          string
	Applied           int
	DuplicateNoop     int
	Quarantined       int
	Failures          int
	IncompleteReasons []string
	Metadata          ReproductionMetadata
	Dispositions      []string
}

// ResetDerivedState deletes only derived M1 platform processing state:
// inbox, inventory_projection, and processing_failures. It never mutates the
// erp schema or broker history.
func ResetDerivedState(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("platform: nil db")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("platform: begin reset: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, q := range []string{
		`DELETE FROM platform.processing_failures`,
		`DELETE FROM platform.inbox`,
		`DELETE FROM platform.inventory_projection`,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("platform: reset derived state: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("platform: commit reset: %w", err)
	}
	return nil
}

// RebuildFromHistory replays retained HistoryRecord values through ProcessRecord
// in the given order. It does not publish, accept ERP orders, or mutate
// authoritative erp state. Incomplete history (residual gaps after replay,
// conflicts, unsupported contracts, other terminal quarantine/failure
// dispositions) yields Status incomplete. Intermediate quarantined_gap rows
// that are re-driven later in the same batch do not by themselves fail the
// rebuild.
func RebuildFromHistory(ctx context.Context, db *sql.DB, records []HistoryRecord, opts RebuildOptions) (RebuildResult, error) {
	if db == nil {
		return RebuildResult{}, fmt.Errorf("platform: nil db")
	}
	if opts.ResetFirst {
		if err := ResetDerivedState(ctx, db); err != nil {
			return RebuildResult{}, err
		}
	}

	out := RebuildResult{
		Status:   RebuildStatusComplete,
		Metadata: opts.Metadata,
	}
	if out.Metadata.HandlerVersion == "" {
		out.Metadata.HandlerVersion = HandlerVersion
	}
	if out.Metadata.EventContractVersion == 0 {
		out.Metadata.EventContractVersion = event.SchemaVersionV1
	}
	if out.Metadata.Seed == "" {
		out.Metadata.Seed = northstar.DefaultSeed
	}
	if out.Metadata.BrokerTopic == "" {
		out.Metadata.BrokerTopic = relay.Topic
	}
	if out.Metadata.ChecksumInputs == "" {
		out.Metadata.ChecksumInputs = "tenant inventory_projection rows: tenant_id, item_id, quantity_on_hand, aggregate_version (CONTRACTS.md §8)"
	}
	if out.Metadata.Limitations == "" {
		out.Metadata.Limitations = "Local library rebuild only; not a hosted FC-014 campaign; not M2 operator recovery or M3 backup/restore."
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
			// Intermediate gaps may be re-driven later in this same batch.
			// Completeness is decided by residual quarantined_gap rows after
			// the full history is processed.
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
			// Malformed/unsupported parse failures use quarantined_invalid via
			// persistParseFailure; other empty dispositions are incomplete.
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

	// Residual open gaps after replay also fail the rebuild campaign.
	var gapCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM platform.inbox
		WHERE consumer_name = $1 AND disposition = $2
	`, ConsumerName, DispositionQuarantinedGap).Scan(&gapCount); err != nil {
		return out, fmt.Errorf("platform: count residual gaps: %w", err)
	}
	if gapCount > 0 {
		out.Status = RebuildStatusIncomplete
		out.IncompleteReasons = append(out.IncompleteReasons,
			fmt.Sprintf("%d residual quarantined_gap row(s)", gapCount))
	}

	var failureCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM platform.processing_failures
		WHERE consumer_name = $1
	`, ConsumerName).Scan(&failureCount); err != nil {
		return out, fmt.Errorf("platform: count processing failures: %w", err)
	}
	if failureCount > 0 {
		out.Failures = failureCount
		out.Status = RebuildStatusIncomplete
		out.IncompleteReasons = append(out.IncompleteReasons,
			fmt.Sprintf("%d processing_failures row(s)", failureCount))
	}

	tenantID := strings.TrimSpace(opts.TenantID)
	if tenantID == "" {
		out.Status = RebuildStatusIncomplete
		out.IncompleteReasons = append(out.IncompleteReasons, "missing tenant_id for checksum")
	} else {
		sum, err := ChecksumTenant(ctx, db, tenantID)
		if err != nil {
			return out, err
		}
		out.Checksum = sum
		out.Metadata.Checksum = sum
	}

	out.Metadata.RebuildStatus = out.Status
	return out, nil
}
