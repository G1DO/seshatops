package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/G1DO/seshatops/event"
)

// ChecksumTenant returns the docs/design/specifications/event-spine.md §8 SHA-256 hex checksum of one
// tenant's complete inventory projection at the current committed snapshot.
// The empty projection hashes the empty byte sequence.
func ChecksumTenant(ctx context.Context, db *sql.DB, tenantID string) (string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT tenant_id, item_id, quantity_on_hand, aggregate_version
		FROM platform.inventory_projection
		WHERE tenant_id = $1
	`, strings.ToLower(tenantID))
	if err != nil {
		return "", fmt.Errorf("platform: checksum query: %w", err)
	}
	defer rows.Close()

	type row struct {
		tenantID string
		itemID   string
		qty      int64
		version  int64
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.tenantID, &r.itemID, &r.qty, &r.version); err != nil {
			return "", fmt.Errorf("platform: checksum scan: %w", err)
		}
		list = append(list, row{
			tenantID: strings.ToLower(r.tenantID),
			itemID:   strings.ToLower(r.itemID),
			qty:      r.qty,
			version:  r.version,
		})
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("platform: checksum rows: %w", err)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].tenantID != list[j].tenantID {
			return list[i].tenantID < list[j].tenantID
		}
		return list[i].itemID < list[j].itemID
	})

	var b strings.Builder
	for _, r := range list {
		b.WriteString(r.tenantID)
		b.WriteByte('\t')
		b.WriteString(r.itemID)
		b.WriteByte('\t')
		b.WriteString(formatInt(r.qty))
		b.WriteByte('\t')
		b.WriteString(formatInt(r.version))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), nil
}

// ChecksumLineage returns the event-spine.md §8 lineage SHA-256 hex checksum of
// one tenant's complete lineage projection. It is independent of ChecksumTenant.
// Empty tenantID fails closed. The empty projection hashes the empty byte sequence.
func ChecksumLineage(ctx context.Context, db *sql.DB, tenantID string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("%w: nil db", ErrTransient)
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if tenantID == "" {
		return "", fmt.Errorf("platform: checksum lineage: empty tenant_id")
	}

	type row struct {
		kind     string
		tenantID string
		hopID    string
		parentID string
		itemID   string
		orderID  string
		version  int64
		eventID  string
		schema   int64
	}
	var list []row
	appendRows := func(kind, query string) error {
		qrows, err := db.QueryContext(ctx, query, tenantID)
		if err != nil {
			return fmt.Errorf("platform: lineage checksum query %s: %w", kind, err)
		}
		defer qrows.Close()
		for qrows.Next() {
			r := row{kind: kind}
			if err := qrows.Scan(&r.tenantID, &r.hopID, &r.parentID, &r.itemID, &r.orderID, &r.version, &r.eventID, &r.schema); err != nil {
				return fmt.Errorf("platform: lineage checksum scan %s: %w", kind, err)
			}
			r.tenantID = strings.ToLower(r.tenantID)
			r.hopID = strings.ToLower(r.hopID)
			r.parentID = strings.ToLower(r.parentID)
			r.itemID = strings.ToLower(r.itemID)
			r.orderID = strings.ToLower(r.orderID)
			r.eventID = strings.ToLower(r.eventID)
			list = append(list, r)
		}
		if err := qrows.Err(); err != nil {
			return fmt.Errorf("platform: lineage checksum rows %s: %w", kind, err)
		}
		return nil
	}

	if err := appendRows(event.AggregateTypeSupplier, `
		SELECT tenant_id, supplier_id, '' AS parent_id, '' AS item_id, '' AS order_id,
		       aggregate_version, source_event_id, event_schema_version
		FROM platform.lineage_suppliers
		WHERE tenant_id = $1
	`); err != nil {
		return "", err
	}
	if err := appendRows(event.AggregateTypeIngredientLot, `
		SELECT tenant_id, lot_id, supplier_id, item_id, '' AS order_id,
		       aggregate_version, source_event_id, event_schema_version
		FROM platform.lineage_ingredient_lots
		WHERE tenant_id = $1
	`); err != nil {
		return "", err
	}
	if err := appendRows(event.AggregateTypeProductionBatch, `
		SELECT tenant_id, batch_id, lot_id, '' AS item_id, '' AS order_id,
		       aggregate_version, source_event_id, event_schema_version
		FROM platform.lineage_production_batches
		WHERE tenant_id = $1
	`); err != nil {
		return "", err
	}
	if err := appendRows(event.AggregateTypeShipment, `
		SELECT tenant_id, shipment_id, batch_id, '' AS item_id, order_id,
		       aggregate_version, source_event_id, event_schema_version
		FROM platform.lineage_shipments
		WHERE tenant_id = $1
	`); err != nil {
		return "", err
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].kind != list[j].kind {
			return list[i].kind < list[j].kind
		}
		if list[i].tenantID != list[j].tenantID {
			return list[i].tenantID < list[j].tenantID
		}
		return list[i].hopID < list[j].hopID
	})

	var b strings.Builder
	for _, r := range list {
		b.WriteString(r.kind)
		b.WriteByte('\t')
		b.WriteString(r.tenantID)
		b.WriteByte('\t')
		b.WriteString(r.hopID)
		b.WriteByte('\t')
		b.WriteString(r.parentID)
		b.WriteByte('\t')
		b.WriteString(r.itemID)
		b.WriteByte('\t')
		b.WriteString(r.orderID)
		b.WriteByte('\t')
		b.WriteString(formatInt(r.version))
		b.WriteByte('\t')
		b.WriteString(r.eventID)
		b.WriteByte('\t')
		b.WriteString(formatInt(r.schema))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), nil
}

func formatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
