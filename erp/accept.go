package erp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/G1DO/seshatops/event"
)

const maxSafeInt = int64(9007199254740991) // 2^53 - 1

// OrderCommand is the minimum one-line synthetic order accepted by M1.
type OrderCommand struct {
	EventID       string
	TenantID      string
	OrderID       string
	ItemID        string
	Quantity      int64
	OccurredAt    string
	RecordedAt    string
	CorrelationID string
	TraceID       string
}

// AcceptResult is the durable outcome of a successful AcceptOrder commit.
type AcceptResult struct {
	OrderID          string
	TenantID         string
	ItemID           string
	QuantityBefore   int64
	QuantityAfter    int64
	AggregateVersion int64
	EventID          string
	ContentHash      string
	EventBytes       []byte
	OutboxStatus     string
}

// testFailBeforeCommit, when set by same-package tests, runs after all writes
// and before Commit. A non-nil error forces Rollback.
var testFailBeforeCommit func(ctx context.Context) error

func setTestFailBeforeCommitForTest(fn func(ctx context.Context) error) {
	testFailBeforeCommit = fn
}

// AcceptOrder validates and accepts one synthetic order, updating authoritative
// inventory and inserting exactly one pending outbox row in one transaction.
// It does not call or require Redpanda.
func AcceptOrder(ctx context.Context, db *sql.DB, cmd OrderCommand) (AcceptResult, error) {
	if err := validateCommand(cmd); err != nil {
		return AcceptResult{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return AcceptResult{}, fmt.Errorf("erp: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		quantityOnHand   int64
		aggregateVersion int64
		rowTenant        string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT tenant_id, quantity_on_hand, aggregate_version
		FROM erp.inventory_items
		WHERE tenant_id = $1 AND item_id = $2
		FOR UPDATE
	`, cmd.TenantID, cmd.ItemID).Scan(&rowTenant, &quantityOnHand, &aggregateVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return AcceptResult{}, ErrUnknownItem
	}
	if err != nil {
		return AcceptResult{}, fmt.Errorf("erp: lock inventory: %w", err)
	}
	if rowTenant != cmd.TenantID {
		return AcceptResult{}, ErrTenantMismatch
	}
	if quantityOnHand < cmd.Quantity {
		return AcceptResult{}, ErrInvalidTransition
	}

	quantityAfter := quantityOnHand - cmd.Quantity
	nextVersion := aggregateVersion + 1
	if nextVersion < 1 || nextVersion > maxSafeInt {
		return AcceptResult{}, ErrInvalidTransition
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE erp.inventory_items
		SET quantity_on_hand = $1, aggregate_version = $2
		WHERE tenant_id = $3 AND item_id = $4
	`, quantityAfter, nextVersion, cmd.TenantID, cmd.ItemID)
	if err != nil {
		return AcceptResult{}, fmt.Errorf("erp: update inventory: %w", err)
	}

	occurredAt, err := parseTimeZ(cmd.OccurredAt)
	if err != nil {
		return AcceptResult{}, ErrTenantMismatch
	}
	recordedAt, err := parseTimeZ(cmd.RecordedAt)
	if err != nil {
		return AcceptResult{}, ErrTenantMismatch
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO erp.orders (
			order_id, tenant_id, item_id, quantity, occurred_at, correlation_id, trace_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, cmd.OrderID, cmd.TenantID, cmd.ItemID, cmd.Quantity, occurredAt, cmd.CorrelationID, cmd.TraceID)
	if err != nil {
		if isUniqueViolation(err) {
			return AcceptResult{}, ErrDuplicateOrder
		}
		return AcceptResult{}, fmt.Errorf("erp: insert order: %w", err)
	}

	env := event.Envelope{
		EventID:            cmd.EventID,
		TenantID:           cmd.TenantID,
		EventType:          event.EventTypeQuantityDecremented,
		EventSchemaVersion: event.SchemaVersionV1,
		AggregateType:      event.AggregateTypeInventoryItem,
		AggregateID:        cmd.ItemID,
		AggregateVersion:   nextVersion,
		OccurredAt:         cmd.OccurredAt,
		RecordedAt:         cmd.RecordedAt,
		Producer:           event.ProducerSyntheticERP,
		CorrelationID:      cmd.CorrelationID,
		CausationID:        nil,
		TraceID:            cmd.TraceID,
		Payload: event.QuantityDecremented{
			OrderID:             cmd.OrderID,
			ItemID:              cmd.ItemID,
			QuantityDecremented: cmd.Quantity,
			QuantityBefore:      quantityOnHand,
			QuantityAfter:       quantityAfter,
		},
	}
	if err := event.Validate(env); err != nil {
		return AcceptResult{}, err
	}
	eventBytes, err := event.CanonicalBytes(env)
	if err != nil {
		return AcceptResult{}, fmt.Errorf("erp: canonical bytes: %w", err)
	}
	contentHash, err := event.ContentHash(env)
	if err != nil {
		return AcceptResult{}, fmt.Errorf("erp: content hash: %w", err)
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
			return AcceptResult{}, ErrDuplicateEvent
		}
		return AcceptResult{}, fmt.Errorf("erp: insert outbox: %w", err)
	}

	if testFailBeforeCommit != nil {
		if err := testFailBeforeCommit(ctx); err != nil {
			return AcceptResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return AcceptResult{}, fmt.Errorf("erp: commit: %w", err)
	}

	return AcceptResult{
		OrderID:          cmd.OrderID,
		TenantID:         cmd.TenantID,
		ItemID:           cmd.ItemID,
		QuantityBefore:   quantityOnHand,
		QuantityAfter:    quantityAfter,
		AggregateVersion: nextVersion,
		EventID:          env.EventID,
		ContentHash:      contentHash,
		EventBytes:       append([]byte(nil), eventBytes...),
		OutboxStatus:     "pending",
	}, nil
}

func validateCommand(cmd OrderCommand) error {
	if cmd.Quantity < 1 || cmd.Quantity > maxSafeInt {
		return ErrInvalidQuantity
	}
	if err := requireLowerUUID(cmd.TenantID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireUUID(cmd.OrderID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireUUID(cmd.EventID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireUUID(cmd.CorrelationID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireCanonicalID(cmd.ItemID); err != nil {
		return ErrTenantMismatch
	}
	if strings.TrimSpace(cmd.TraceID) == "" || !utf8.ValidString(cmd.TraceID) {
		return ErrTenantMismatch
	}
	if _, err := parseTimeZ(cmd.OccurredAt); err != nil {
		return ErrTenantMismatch
	}
	if _, err := parseTimeZ(cmd.RecordedAt); err != nil {
		return ErrTenantMismatch
	}
	return nil
}

func requireUUID(v string) error {
	if !utf8.ValidString(v) {
		return fmt.Errorf("invalid utf-8")
	}
	if len(v) != 36 {
		return fmt.Errorf("invalid uuid")
	}
	lower := strings.ToLower(v)
	for i, c := range lower {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return fmt.Errorf("invalid uuid")
			}
		default:
			if !isHex(c) {
				return fmt.Errorf("invalid uuid")
			}
		}
	}
	if lower[14] != '4' {
		return fmt.Errorf("invalid uuid")
	}
	switch lower[19] {
	case '8', '9', 'a', 'b':
	default:
		return fmt.Errorf("invalid uuid")
	}
	return nil
}

func requireLowerUUID(v string) error {
	if err := requireUUID(v); err != nil {
		return err
	}
	if v != strings.ToLower(v) {
		return fmt.Errorf("tenant must be lowercase")
	}
	return nil
}

func requireCanonicalID(v string) error {
	if v == "" || !utf8.ValidString(v) {
		return fmt.Errorf("invalid id")
	}
	if v != strings.ToLower(v) {
		return fmt.Errorf("id must be lowercase")
	}
	for _, r := range v {
		if unicode.IsSpace(r) {
			return fmt.Errorf("id contains whitespace")
		}
	}
	return nil
}

func parseTimeZ(v string) (time.Time, error) {
	if !utf8.ValidString(v) {
		return time.Time{}, fmt.Errorf("invalid time")
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}, err
	}
	if !strings.HasSuffix(v, "Z") {
		return time.Time{}, fmt.Errorf("time must use Z")
	}
	return t.UTC(), nil
}

func isHex(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
}

func isUniqueViolation(err error) bool {
	// Avoid importing pgconn solely for error codes; PostgreSQL unique_violation is 23505.
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "duplicate key")
}
