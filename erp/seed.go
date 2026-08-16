package erp

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
)

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
