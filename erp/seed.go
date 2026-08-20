package erp

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
)

// ResetSourceForTenant deletes synthetic ERP source and outbox state for one
// tenant. Callers must provide their own explicit disposable-environment
// safety gate; this helper never runs automatically.
func ResetSourceForTenant(ctx context.Context, db *sql.DB, tenantID string) error {
	if db == nil {
		return fmt.Errorf("erp: reset source: nil db")
	}
	if tenantID == "" {
		return fmt.Errorf("erp: reset source: tenant_id required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("erp: begin source reset: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, query := range []string{
		`DELETE FROM erp.outbox WHERE tenant_id = $1`,
		`DELETE FROM erp.shipments WHERE tenant_id = $1`,
		`DELETE FROM erp.production_batches WHERE tenant_id = $1`,
		`DELETE FROM erp.ingredient_lots WHERE tenant_id = $1`,
		`DELETE FROM erp.orders WHERE tenant_id = $1`,
		`DELETE FROM erp.suppliers WHERE tenant_id = $1`,
		`DELETE FROM erp.inventory_items WHERE tenant_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, query, tenantID); err != nil {
			return fmt.Errorf("erp: reset source: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erp: commit source reset: %w", err)
	}
	return nil
}

// SeedNorthstarInventory inserts the authoritative inventory row for the
// declared Northstar Event Spine fixture. Aggregate version starts at 0 so the first
// accepted order emits aggregate_version 1.
func SeedNorthstarInventory(ctx context.Context, db *sql.DB, fx northstar.Fixture) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO erp.inventory_items (tenant_id, item_id, quantity_on_hand, aggregate_version)
		VALUES ($1, $2, $3, 0)
	`, fx.TenantID, fx.ItemID, fx.QuantityBefore)
	if err != nil {
		return fmt.Errorf("erp: seed inventory: %w", err)
	}
	return nil
}

// SeedLineageInventory inserts the authoritative inventory row for the Northstar
// M3 lineage fixture. Aggregate version starts at 0 so the inventory hop emits
// aggregate_version 1.
func SeedLineageInventory(ctx context.Context, db *sql.DB, fx northstar.LineageFixture) error {
	p, err := lineageInventoryPayload(fx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO erp.inventory_items (tenant_id, item_id, quantity_on_hand, aggregate_version)
		VALUES ($1, $2, $3, 0)
	`, fx.TenantID, fx.ItemID, p.QuantityBefore)
	if err != nil {
		return fmt.Errorf("erp: seed lineage inventory: %w", err)
	}
	return nil
}

// OrderCommandFromFixture builds the AcceptOrder command that reproduces the
// Issue #22 Northstar logical event when inventory is seeded from the same fixture.
func OrderCommandFromFixture(fx northstar.Fixture) (OrderCommand, error) {
	p, ok := event.AsQuantityDecremented(fx.Event)
	if !ok {
		return OrderCommand{}, ErrInvalidFixture
	}
	return OrderCommand{
		EventID:       fx.Event.EventID,
		TenantID:      fx.TenantID,
		OrderID:       fx.OrderID,
		ItemID:        fx.ItemID,
		Quantity:      p.QuantityDecremented,
		OccurredAt:    fx.Event.OccurredAt,
		RecordedAt:    fx.Event.RecordedAt,
		CorrelationID: fx.Event.CorrelationID,
		TraceID:       fx.Event.TraceID,
	}, nil
}

// SupplierCommandFromLineage builds RegisterSupplier from the Northstar lineage fixture.
func SupplierCommandFromLineage(fx northstar.LineageFixture) (SupplierCommand, error) {
	env, err := lineageEvent(fx, 0, event.EventTypeSupplierRegistered)
	if err != nil {
		return SupplierCommand{}, err
	}
	p, ok := env.Payload.(event.SupplierRegistered)
	if !ok {
		return SupplierCommand{}, ErrInvalidFixture
	}
	return SupplierCommand{
		EventID:       env.EventID,
		TenantID:      env.TenantID,
		SupplierID:    p.SupplierID,
		OccurredAt:    env.OccurredAt,
		RecordedAt:    env.RecordedAt,
		CorrelationID: env.CorrelationID,
		CausationID:   copyCausation(env.CausationID),
		TraceID:       env.TraceID,
	}, nil
}

// IngredientLotCommandFromLineage builds ReceiveIngredientLot from the lineage fixture.
func IngredientLotCommandFromLineage(fx northstar.LineageFixture) (IngredientLotCommand, error) {
	env, err := lineageEvent(fx, 1, event.EventTypeIngredientLotReceived)
	if err != nil {
		return IngredientLotCommand{}, err
	}
	p, ok := env.Payload.(event.IngredientLotReceived)
	if !ok {
		return IngredientLotCommand{}, ErrInvalidFixture
	}
	return IngredientLotCommand{
		EventID:       env.EventID,
		TenantID:      env.TenantID,
		LotID:         p.LotID,
		SupplierID:    p.SupplierID,
		ItemID:        p.ItemID,
		OccurredAt:    env.OccurredAt,
		RecordedAt:    env.RecordedAt,
		CorrelationID: env.CorrelationID,
		CausationID:   copyCausation(env.CausationID),
		TraceID:       env.TraceID,
	}, nil
}

// ProductionBatchCommandFromLineage builds ProduceProductionBatch from the lineage fixture.
func ProductionBatchCommandFromLineage(fx northstar.LineageFixture) (ProductionBatchCommand, error) {
	env, err := lineageEvent(fx, 2, event.EventTypeProductionBatchProduced)
	if err != nil {
		return ProductionBatchCommand{}, err
	}
	p, ok := env.Payload.(event.ProductionBatchProduced)
	if !ok {
		return ProductionBatchCommand{}, ErrInvalidFixture
	}
	return ProductionBatchCommand{
		EventID:       env.EventID,
		TenantID:      env.TenantID,
		BatchID:       p.BatchID,
		LotID:         p.LotID,
		OccurredAt:    env.OccurredAt,
		RecordedAt:    env.RecordedAt,
		CorrelationID: env.CorrelationID,
		CausationID:   copyCausation(env.CausationID),
		TraceID:       env.TraceID,
	}, nil
}

// ShipmentCommandFromLineage builds DispatchShipment from the lineage fixture.
func ShipmentCommandFromLineage(fx northstar.LineageFixture) (ShipmentCommand, error) {
	env, err := lineageEvent(fx, 3, event.EventTypeShipmentDispatched)
	if err != nil {
		return ShipmentCommand{}, err
	}
	p, ok := env.Payload.(event.ShipmentDispatched)
	if !ok {
		return ShipmentCommand{}, ErrInvalidFixture
	}
	return ShipmentCommand{
		EventID:       env.EventID,
		TenantID:      env.TenantID,
		ShipmentID:    p.ShipmentID,
		BatchID:       p.BatchID,
		OrderID:       p.OrderID,
		OccurredAt:    env.OccurredAt,
		RecordedAt:    env.RecordedAt,
		CorrelationID: env.CorrelationID,
		CausationID:   copyCausation(env.CausationID),
		TraceID:       env.TraceID,
	}, nil
}

// OrderCommandFromLineage builds AcceptOrder from the lineage inventory hop,
// including causation_id pointing at the shipment event.
func OrderCommandFromLineage(fx northstar.LineageFixture) (OrderCommand, error) {
	env, err := lineageEvent(fx, 4, event.EventTypeQuantityDecremented)
	if err != nil {
		return OrderCommand{}, err
	}
	p, ok := event.AsQuantityDecremented(env)
	if !ok {
		return OrderCommand{}, ErrInvalidFixture
	}
	return OrderCommand{
		EventID:       env.EventID,
		TenantID:      env.TenantID,
		OrderID:       p.OrderID,
		ItemID:        p.ItemID,
		Quantity:      p.QuantityDecremented,
		OccurredAt:    env.OccurredAt,
		RecordedAt:    env.RecordedAt,
		CorrelationID: env.CorrelationID,
		CausationID:   copyCausation(env.CausationID),
		TraceID:       env.TraceID,
	}, nil
}

func lineageEvent(fx northstar.LineageFixture, i int, wantType string) (event.Envelope, error) {
	if i < 0 || i >= len(fx.Events) {
		return event.Envelope{}, ErrInvalidFixture
	}
	env := fx.Events[i]
	if env.EventType != wantType {
		return event.Envelope{}, ErrInvalidFixture
	}
	return env, nil
}

func lineageInventoryPayload(fx northstar.LineageFixture) (event.QuantityDecremented, error) {
	env, err := lineageEvent(fx, 4, event.EventTypeQuantityDecremented)
	if err != nil {
		return event.QuantityDecremented{}, err
	}
	p, ok := event.AsQuantityDecremented(env)
	if !ok {
		return event.QuantityDecremented{}, ErrInvalidFixture
	}
	return p, nil
}

func copyCausation(id *string) *string {
	if id == nil {
		return nil
	}
	v := *id
	return &v
}
