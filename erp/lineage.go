package erp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/G1DO/seshatops/event"
)

const firstAggregateVersion int64 = 1

// SourceAcceptResult is the durable outcome of a successful lineage source hop.
type SourceAcceptResult struct {
	AggregateID      string
	AggregateType    string
	AggregateVersion int64
	EventID          string
	ContentHash      string
	EventBytes       []byte
	OutboxStatus     string
}

// SupplierCommand registers one synthetic supplier.
type SupplierCommand struct {
	EventID       string
	TenantID      string
	SupplierID    string
	OccurredAt    string
	RecordedAt    string
	CorrelationID string
	CausationID   *string
	TraceID       string
}

// IngredientLotCommand records one synthetic ingredient lot.
type IngredientLotCommand struct {
	EventID       string
	TenantID      string
	LotID         string
	SupplierID    string
	ItemID        string
	OccurredAt    string
	RecordedAt    string
	CorrelationID string
	CausationID   *string
	TraceID       string
}

// ProductionBatchCommand records one synthetic production batch.
type ProductionBatchCommand struct {
	EventID       string
	TenantID      string
	BatchID       string
	LotID         string
	OccurredAt    string
	RecordedAt    string
	CorrelationID string
	CausationID   *string
	TraceID       string
}

// ShipmentCommand records one synthetic shipment. OrderID is stored without
// requiring erp.orders to exist; the M3 fixture dispatches before AcceptOrder.
type ShipmentCommand struct {
	EventID       string
	TenantID      string
	ShipmentID    string
	BatchID       string
	OrderID       string
	OccurredAt    string
	RecordedAt    string
	CorrelationID string
	CausationID   *string
	TraceID       string
}

// RegisterSupplier inserts one supplier source row and a pending outbox event.
func RegisterSupplier(ctx context.Context, db *sql.DB, cmd SupplierCommand) (SourceAcceptResult, error) {
	if err := validateSupplierCommand(cmd); err != nil {
		return SourceAcceptResult{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SourceAcceptResult{}, fmt.Errorf("erp: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	registeredAt, err := parseTimeZ(cmd.OccurredAt)
	if err != nil {
		return SourceAcceptResult{}, ErrTenantMismatch
	}
	recordedAt, err := parseTimeZ(cmd.RecordedAt)
	if err != nil {
		return SourceAcceptResult{}, ErrTenantMismatch
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO erp.suppliers (
			tenant_id, supplier_id, aggregate_version, registered_at, correlation_id, trace_id
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, cmd.TenantID, cmd.SupplierID, firstAggregateVersion, registeredAt, cmd.CorrelationID, cmd.TraceID)
	if err != nil {
		if isUniqueViolation(err) {
			return SourceAcceptResult{}, ErrDuplicateSource
		}
		return SourceAcceptResult{}, fmt.Errorf("erp: insert supplier: %w", err)
	}

	env := event.Envelope{
		EventID:            cmd.EventID,
		TenantID:           cmd.TenantID,
		EventType:          event.EventTypeSupplierRegistered,
		EventSchemaVersion: event.SchemaVersionV1,
		AggregateType:      event.AggregateTypeSupplier,
		AggregateID:        cmd.SupplierID,
		AggregateVersion:   firstAggregateVersion,
		OccurredAt:         cmd.OccurredAt,
		RecordedAt:         cmd.RecordedAt,
		Producer:           event.ProducerSyntheticERP,
		CorrelationID:      cmd.CorrelationID,
		CausationID:        nil,
		TraceID:            cmd.TraceID,
		Payload:            event.SupplierRegistered{SupplierID: cmd.SupplierID},
	}
	return finishSourceHop(ctx, tx, env, recordedAt)
}

// ReceiveIngredientLot inserts one lot sourced from an existing supplier.
func ReceiveIngredientLot(ctx context.Context, db *sql.DB, cmd IngredientLotCommand) (SourceAcceptResult, error) {
	if err := validateIngredientLotCommand(cmd); err != nil {
		return SourceAcceptResult{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SourceAcceptResult{}, fmt.Errorf("erp: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockTenantRow(ctx, tx, `
		SELECT tenant_id FROM erp.suppliers
		WHERE tenant_id = $1 AND supplier_id = $2
		FOR UPDATE
	`, cmd.TenantID, cmd.SupplierID, ErrUnknownSource, "supplier"); err != nil {
		return SourceAcceptResult{}, err
	}
	if err := lockTenantRow(ctx, tx, `
		SELECT tenant_id FROM erp.inventory_items
		WHERE tenant_id = $1 AND item_id = $2
		FOR UPDATE
	`, cmd.TenantID, cmd.ItemID, ErrUnknownItem, "inventory"); err != nil {
		return SourceAcceptResult{}, err
	}

	parentID, err := parentOutboxEventID(ctx, tx, cmd.TenantID, event.AggregateTypeSupplier, cmd.SupplierID)
	if err != nil {
		return SourceAcceptResult{}, err
	}
	if err := requireCausation(cmd.CausationID, parentID); err != nil {
		return SourceAcceptResult{}, err
	}

	receivedAt, err := parseTimeZ(cmd.OccurredAt)
	if err != nil {
		return SourceAcceptResult{}, ErrTenantMismatch
	}
	recordedAt, err := parseTimeZ(cmd.RecordedAt)
	if err != nil {
		return SourceAcceptResult{}, ErrTenantMismatch
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO erp.ingredient_lots (
			tenant_id, lot_id, supplier_id, item_id, aggregate_version,
			received_at, correlation_id, trace_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, cmd.TenantID, cmd.LotID, cmd.SupplierID, cmd.ItemID, firstAggregateVersion,
		receivedAt, cmd.CorrelationID, cmd.TraceID)
	if err != nil {
		if isUniqueViolation(err) {
			return SourceAcceptResult{}, ErrDuplicateSource
		}
		return SourceAcceptResult{}, fmt.Errorf("erp: insert ingredient lot: %w", err)
	}

	env := event.Envelope{
		EventID:            cmd.EventID,
		TenantID:           cmd.TenantID,
		EventType:          event.EventTypeIngredientLotReceived,
		EventSchemaVersion: event.SchemaVersionV1,
		AggregateType:      event.AggregateTypeIngredientLot,
		AggregateID:        cmd.LotID,
		AggregateVersion:   firstAggregateVersion,
		OccurredAt:         cmd.OccurredAt,
		RecordedAt:         cmd.RecordedAt,
		Producer:           event.ProducerSyntheticERP,
		CorrelationID:      cmd.CorrelationID,
		CausationID:        cmd.CausationID,
		TraceID:            cmd.TraceID,
		Payload: event.IngredientLotReceived{
			LotID:      cmd.LotID,
			SupplierID: cmd.SupplierID,
			ItemID:     cmd.ItemID,
		},
	}
	return finishSourceHop(ctx, tx, env, recordedAt)
}

// ProduceProductionBatch inserts one batch sourced from an existing lot.
func ProduceProductionBatch(ctx context.Context, db *sql.DB, cmd ProductionBatchCommand) (SourceAcceptResult, error) {
	if err := validateProductionBatchCommand(cmd); err != nil {
		return SourceAcceptResult{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SourceAcceptResult{}, fmt.Errorf("erp: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockTenantRow(ctx, tx, `
		SELECT tenant_id FROM erp.ingredient_lots
		WHERE tenant_id = $1 AND lot_id = $2
		FOR UPDATE
	`, cmd.TenantID, cmd.LotID, ErrUnknownSource, "ingredient lot"); err != nil {
		return SourceAcceptResult{}, err
	}

	parentID, err := parentOutboxEventID(ctx, tx, cmd.TenantID, event.AggregateTypeIngredientLot, cmd.LotID)
	if err != nil {
		return SourceAcceptResult{}, err
	}
	if err := requireCausation(cmd.CausationID, parentID); err != nil {
		return SourceAcceptResult{}, err
	}

	producedAt, err := parseTimeZ(cmd.OccurredAt)
	if err != nil {
		return SourceAcceptResult{}, ErrTenantMismatch
	}
	recordedAt, err := parseTimeZ(cmd.RecordedAt)
	if err != nil {
		return SourceAcceptResult{}, ErrTenantMismatch
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO erp.production_batches (
			tenant_id, batch_id, lot_id, aggregate_version, produced_at, correlation_id, trace_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, cmd.TenantID, cmd.BatchID, cmd.LotID, firstAggregateVersion, producedAt, cmd.CorrelationID, cmd.TraceID)
	if err != nil {
		if isUniqueViolation(err) {
			return SourceAcceptResult{}, ErrDuplicateSource
		}
		return SourceAcceptResult{}, fmt.Errorf("erp: insert production batch: %w", err)
	}

	env := event.Envelope{
		EventID:            cmd.EventID,
		TenantID:           cmd.TenantID,
		EventType:          event.EventTypeProductionBatchProduced,
		EventSchemaVersion: event.SchemaVersionV1,
		AggregateType:      event.AggregateTypeProductionBatch,
		AggregateID:        cmd.BatchID,
		AggregateVersion:   firstAggregateVersion,
		OccurredAt:         cmd.OccurredAt,
		RecordedAt:         cmd.RecordedAt,
		Producer:           event.ProducerSyntheticERP,
		CorrelationID:      cmd.CorrelationID,
		CausationID:        cmd.CausationID,
		TraceID:            cmd.TraceID,
		Payload: event.ProductionBatchProduced{
			BatchID: cmd.BatchID,
			LotID:   cmd.LotID,
		},
	}
	return finishSourceHop(ctx, tx, env, recordedAt)
}

// DispatchShipment inserts one shipment sourced from an existing batch.
func DispatchShipment(ctx context.Context, db *sql.DB, cmd ShipmentCommand) (SourceAcceptResult, error) {
	if err := validateShipmentCommand(cmd); err != nil {
		return SourceAcceptResult{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SourceAcceptResult{}, fmt.Errorf("erp: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockTenantRow(ctx, tx, `
		SELECT tenant_id FROM erp.production_batches
		WHERE tenant_id = $1 AND batch_id = $2
		FOR UPDATE
	`, cmd.TenantID, cmd.BatchID, ErrUnknownSource, "production batch"); err != nil {
		return SourceAcceptResult{}, err
	}

	parentID, err := parentOutboxEventID(ctx, tx, cmd.TenantID, event.AggregateTypeProductionBatch, cmd.BatchID)
	if err != nil {
		return SourceAcceptResult{}, err
	}
	if err := requireCausation(cmd.CausationID, parentID); err != nil {
		return SourceAcceptResult{}, err
	}

	dispatchedAt, err := parseTimeZ(cmd.OccurredAt)
	if err != nil {
		return SourceAcceptResult{}, ErrTenantMismatch
	}
	recordedAt, err := parseTimeZ(cmd.RecordedAt)
	if err != nil {
		return SourceAcceptResult{}, ErrTenantMismatch
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO erp.shipments (
			tenant_id, shipment_id, batch_id, order_id, aggregate_version,
			dispatched_at, correlation_id, trace_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, cmd.TenantID, cmd.ShipmentID, cmd.BatchID, cmd.OrderID, firstAggregateVersion,
		dispatchedAt, cmd.CorrelationID, cmd.TraceID)
	if err != nil {
		if isUniqueViolation(err) {
			return SourceAcceptResult{}, ErrDuplicateSource
		}
		return SourceAcceptResult{}, fmt.Errorf("erp: insert shipment: %w", err)
	}

	env := event.Envelope{
		EventID:            cmd.EventID,
		TenantID:           cmd.TenantID,
		EventType:          event.EventTypeShipmentDispatched,
		EventSchemaVersion: event.SchemaVersionV1,
		AggregateType:      event.AggregateTypeShipment,
		AggregateID:        cmd.ShipmentID,
		AggregateVersion:   firstAggregateVersion,
		OccurredAt:         cmd.OccurredAt,
		RecordedAt:         cmd.RecordedAt,
		Producer:           event.ProducerSyntheticERP,
		CorrelationID:      cmd.CorrelationID,
		CausationID:        cmd.CausationID,
		TraceID:            cmd.TraceID,
		Payload: event.ShipmentDispatched{
			ShipmentID: cmd.ShipmentID,
			BatchID:    cmd.BatchID,
			OrderID:    cmd.OrderID,
		},
	}
	return finishSourceHop(ctx, tx, env, recordedAt)
}

func finishSourceHop(ctx context.Context, tx *sql.Tx, env event.Envelope, recordedAt time.Time) (SourceAcceptResult, error) {
	eventBytes, contentHash, err := insertPendingOutbox(ctx, tx, env, recordedAt)
	if err != nil {
		return SourceAcceptResult{}, err
	}
	if err := commitSource(ctx, tx); err != nil {
		return SourceAcceptResult{}, err
	}
	return SourceAcceptResult{
		AggregateID:      env.AggregateID,
		AggregateType:    env.AggregateType,
		AggregateVersion: env.AggregateVersion,
		EventID:          env.EventID,
		ContentHash:      contentHash,
		EventBytes:       append([]byte(nil), eventBytes...),
		OutboxStatus:     "pending",
	}, nil
}

func lockTenantRow(ctx context.Context, tx *sql.Tx, query, tenantID, id string, missing error, what string) error {
	var rowTenant string
	err := tx.QueryRowContext(ctx, query, tenantID, id).Scan(&rowTenant)
	if errors.Is(err, sql.ErrNoRows) {
		return missing
	}
	if err != nil {
		return fmt.Errorf("erp: lock %s: %w", what, err)
	}
	if rowTenant != tenantID {
		return ErrTenantMismatch
	}
	return nil
}

func validateSupplierCommand(cmd SupplierCommand) error {
	if err := validateHopIdentity(cmd.EventID, cmd.TenantID, cmd.CorrelationID, cmd.TraceID, cmd.OccurredAt, cmd.RecordedAt); err != nil {
		return err
	}
	if err := requireCanonicalID(cmd.SupplierID); err != nil {
		return ErrTenantMismatch
	}
	if cmd.CausationID != nil {
		if err := requireUUID(*cmd.CausationID); err != nil {
			return ErrTenantMismatch
		}
		return ErrInvalidTransition
	}
	return nil
}

func validateIngredientLotCommand(cmd IngredientLotCommand) error {
	if err := validateHopIdentity(cmd.EventID, cmd.TenantID, cmd.CorrelationID, cmd.TraceID, cmd.OccurredAt, cmd.RecordedAt); err != nil {
		return err
	}
	if err := requireCanonicalID(cmd.LotID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireCanonicalID(cmd.SupplierID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireCanonicalID(cmd.ItemID); err != nil {
		return ErrTenantMismatch
	}
	return requireRequiredCausation(cmd.CausationID)
}

func validateProductionBatchCommand(cmd ProductionBatchCommand) error {
	if err := validateHopIdentity(cmd.EventID, cmd.TenantID, cmd.CorrelationID, cmd.TraceID, cmd.OccurredAt, cmd.RecordedAt); err != nil {
		return err
	}
	if err := requireCanonicalID(cmd.BatchID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireCanonicalID(cmd.LotID); err != nil {
		return ErrTenantMismatch
	}
	return requireRequiredCausation(cmd.CausationID)
}

func validateShipmentCommand(cmd ShipmentCommand) error {
	if err := validateHopIdentity(cmd.EventID, cmd.TenantID, cmd.CorrelationID, cmd.TraceID, cmd.OccurredAt, cmd.RecordedAt); err != nil {
		return err
	}
	if err := requireCanonicalID(cmd.ShipmentID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireCanonicalID(cmd.BatchID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireUUID(cmd.OrderID); err != nil {
		return ErrTenantMismatch
	}
	return requireRequiredCausation(cmd.CausationID)
}

func validateHopIdentity(eventID, tenantID, correlationID, traceID, occurredAt, recordedAt string) error {
	if err := requireLowerUUID(tenantID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireUUID(eventID); err != nil {
		return ErrTenantMismatch
	}
	if err := requireUUID(correlationID); err != nil {
		return ErrTenantMismatch
	}
	if strings.TrimSpace(traceID) == "" || !utf8.ValidString(traceID) {
		return ErrTenantMismatch
	}
	if _, err := parseTimeZ(occurredAt); err != nil {
		return ErrTenantMismatch
	}
	if _, err := parseTimeZ(recordedAt); err != nil {
		return ErrTenantMismatch
	}
	return nil
}

func requireRequiredCausation(id *string) error {
	if id == nil {
		return ErrInvalidTransition
	}
	if err := requireUUID(*id); err != nil {
		return ErrTenantMismatch
	}
	return nil
}
