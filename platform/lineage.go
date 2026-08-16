package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/G1DO/seshatops/event"
)

// LineageHop is one tenant-scoped projected lineage node plus source provenance.
type LineageHop struct {
	ID                 string
	ParentID           string
	ItemID             string
	OrderID            string
	AggregateVersion   int64
	SourceEventID      string
	EventSchemaVersion int64
	OccurredAt         string
	RecordedAt         string
	CorrelationID      string
	CausationID        *string
	TraceID            string
}

// BatchTrace is the supplier → lot → batch → shipment → order chain for one
// tenant-scoped production batch. Missing hops are zero values; tenant is never
// taken from resource identifiers.
type BatchTrace struct {
	TenantID string
	Supplier LineageHop
	Lot      LineageHop
	Batch    LineageHop
	Shipment LineageHop
}

func isProjectionFamily(eventType string) bool {
	switch eventType {
	case event.EventTypeQuantityDecremented,
		event.EventTypeSupplierRegistered,
		event.EventTypeIngredientLotReceived,
		event.EventTypeProductionBatchProduced,
		event.EventTypeShipmentDispatched:
		return true
	default:
		return false
	}
}

func projectionSelfID(env event.Envelope) (string, bool) {
	switch env.EventType {
	case event.EventTypeQuantityDecremented:
		p, ok := event.AsQuantityDecremented(env)
		return p.ItemID, ok
	case event.EventTypeSupplierRegistered:
		p, ok := event.AsSupplierRegistered(env)
		return p.SupplierID, ok
	case event.EventTypeIngredientLotReceived:
		p, ok := event.AsIngredientLotReceived(env)
		return p.LotID, ok
	case event.EventTypeProductionBatchProduced:
		p, ok := event.AsProductionBatchProduced(env)
		return p.BatchID, ok
	case event.EventTypeShipmentDispatched:
		p, ok := event.AsShipmentDispatched(env)
		return p.ShipmentID, ok
	default:
		return "", false
	}
}

func applyEvent(ctx context.Context, tx *sql.Tx, env event.Envelope, hash string, canonical []byte) (string, error) {
	switch env.EventType {
	case event.EventTypeQuantityDecremented:
		return applyProjection(ctx, tx, env, hash, canonical)
	case event.EventTypeSupplierRegistered,
		event.EventTypeIngredientLotReceived,
		event.EventTypeProductionBatchProduced,
		event.EventTypeShipmentDispatched:
		return applyLineage(ctx, tx, env, hash, canonical)
	default:
		return "", fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
	}
}

func applyLineage(ctx context.Context, tx *sql.Tx, env event.Envelope, hash string, canonical []byte) (string, error) {
	current, found, err := lockLineageVersion(ctx, tx, env)
	if err != nil {
		return "", err
	}
	if found {
		disp := lineageExistingDisposition(current, env.AggregateVersion)
		if err := upsertInbox(ctx, tx, inboxRow{
			EventID:          env.EventID,
			TenantID:         env.TenantID,
			ContentHash:      hash,
			AggregateType:    env.AggregateType,
			AggregateID:      env.AggregateID,
			AggregateVersion: env.AggregateVersion,
			Disposition:      disp,
			ExpectedVersion:  ptrInt64(current + 1),
			ReceivedVersion:  ptrInt64(env.AggregateVersion),
			EventBytes:       gapBytes(disp, canonical),
		}); err != nil {
			return "", err
		}
		return disp, nil
	}
	if env.AggregateVersion != 1 {
		if err := upsertInbox(ctx, tx, inboxRow{
			EventID:          env.EventID,
			TenantID:         env.TenantID,
			ContentHash:      hash,
			AggregateType:    env.AggregateType,
			AggregateID:      env.AggregateID,
			AggregateVersion: env.AggregateVersion,
			Disposition:      DispositionQuarantinedGap,
			ExpectedVersion:  ptrInt64(1),
			ReceivedVersion:  ptrInt64(env.AggregateVersion),
			EventBytes:       canonical,
		}); err != nil {
			return "", err
		}
		return DispositionQuarantinedGap, nil
	}

	inserted, err := insertLineageHop(ctx, tx, env)
	if err != nil {
		return "", err
	}
	if !inserted {
		existingID, found, err := lineageSourceEventID(ctx, tx, env)
		if err != nil {
			return "", err
		}
		disp := DispositionQuarantinedInvalid
		if found && existingID == env.EventID {
			// Concurrent identical delivery already projected this hop.
			disp = DispositionDuplicateNoop
		}
		if err := upsertInbox(ctx, tx, inboxRow{
			EventID:          env.EventID,
			TenantID:         env.TenantID,
			ContentHash:      hash,
			AggregateType:    env.AggregateType,
			AggregateID:      env.AggregateID,
			AggregateVersion: env.AggregateVersion,
			Disposition:      disp,
		}); err != nil {
			return "", err
		}
		return disp, nil
	}
	if err := upsertInbox(ctx, tx, inboxRow{
		EventID:          env.EventID,
		TenantID:         env.TenantID,
		ContentHash:      hash,
		AggregateType:    env.AggregateType,
		AggregateID:      env.AggregateID,
		AggregateVersion: env.AggregateVersion,
		Disposition:      DispositionApplied,
	}); err != nil {
		return "", err
	}
	return DispositionApplied, nil
}

func lineageExistingDisposition(current, incoming int64) string {
	if incoming <= current {
		return DispositionQuarantinedStale
	}
	if incoming > current+1 {
		return DispositionQuarantinedGap
	}
	// Contiguous next version has no lineage-update contract in this slice.
	return DispositionQuarantinedInvalid
}

func gapBytes(disp string, canonical []byte) []byte {
	if disp == DispositionQuarantinedGap {
		return canonical
	}
	return nil
}

func lockLineageVersion(ctx context.Context, tx *sql.Tx, env event.Envelope) (version int64, found bool, err error) {
	query, err := lineageLockQuery(env.EventType)
	if err != nil {
		return 0, false, err
	}
	err = tx.QueryRowContext(ctx, query, env.TenantID, env.AggregateID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("%w: lock lineage: %v", ErrTransient, err)
	}
	return version, true, nil
}

func lineageSourceEventID(ctx context.Context, tx *sql.Tx, env event.Envelope) (string, bool, error) {
	query, err := lineageSourceEventQuery(env.EventType)
	if err != nil {
		return "", false, err
	}
	var id string
	err = tx.QueryRowContext(ctx, query, env.TenantID, env.AggregateID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("%w: lineage source event: %v", ErrTransient, err)
	}
	return id, true, nil
}

func lineageSourceEventQuery(eventType string) (string, error) {
	switch eventType {
	case event.EventTypeSupplierRegistered:
		return `
			SELECT source_event_id
			FROM platform.lineage_suppliers
			WHERE tenant_id = $1 AND supplier_id = $2
		`, nil
	case event.EventTypeIngredientLotReceived:
		return `
			SELECT source_event_id
			FROM platform.lineage_ingredient_lots
			WHERE tenant_id = $1 AND lot_id = $2
		`, nil
	case event.EventTypeProductionBatchProduced:
		return `
			SELECT source_event_id
			FROM platform.lineage_production_batches
			WHERE tenant_id = $1 AND batch_id = $2
		`, nil
	case event.EventTypeShipmentDispatched:
		return `
			SELECT source_event_id
			FROM platform.lineage_shipments
			WHERE tenant_id = $1 AND shipment_id = $2
		`, nil
	default:
		return "", fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
	}
}

func lineageLockQuery(eventType string) (string, error) {
	switch eventType {
	case event.EventTypeSupplierRegistered:
		return `
			SELECT aggregate_version
			FROM platform.lineage_suppliers
			WHERE tenant_id = $1 AND supplier_id = $2
			FOR UPDATE
		`, nil
	case event.EventTypeIngredientLotReceived:
		return `
			SELECT aggregate_version
			FROM platform.lineage_ingredient_lots
			WHERE tenant_id = $1 AND lot_id = $2
			FOR UPDATE
		`, nil
	case event.EventTypeProductionBatchProduced:
		return `
			SELECT aggregate_version
			FROM platform.lineage_production_batches
			WHERE tenant_id = $1 AND batch_id = $2
			FOR UPDATE
		`, nil
	case event.EventTypeShipmentDispatched:
		return `
			SELECT aggregate_version
			FROM platform.lineage_shipments
			WHERE tenant_id = $1 AND shipment_id = $2
			FOR UPDATE
		`, nil
	default:
		return "", fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
	}
}

func insertLineageHop(ctx context.Context, tx *sql.Tx, env event.Envelope) (inserted bool, err error) {
	causation := nullString(env.CausationID)
	var res sql.Result
	switch env.EventType {
	case event.EventTypeSupplierRegistered:
		p, ok := event.AsSupplierRegistered(env)
		if !ok {
			return false, fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
		}
		res, err = tx.ExecContext(ctx, `
			INSERT INTO platform.lineage_suppliers (
				tenant_id, supplier_id, aggregate_version,
				source_event_id, event_schema_version,
				occurred_at, recorded_at, correlation_id, causation_id, trace_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT DO NOTHING
		`, env.TenantID, p.SupplierID, env.AggregateVersion,
			env.EventID, env.EventSchemaVersion,
			env.OccurredAt, env.RecordedAt, env.CorrelationID, causation, env.TraceID)
		if err != nil {
			return false, fmt.Errorf("%w: insert lineage supplier: %v", ErrTransient, err)
		}
	case event.EventTypeIngredientLotReceived:
		p, ok := event.AsIngredientLotReceived(env)
		if !ok {
			return false, fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
		}
		res, err = tx.ExecContext(ctx, `
			INSERT INTO platform.lineage_ingredient_lots (
				tenant_id, lot_id, supplier_id, item_id, aggregate_version,
				source_event_id, event_schema_version,
				occurred_at, recorded_at, correlation_id, causation_id, trace_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT DO NOTHING
		`, env.TenantID, p.LotID, p.SupplierID, p.ItemID, env.AggregateVersion,
			env.EventID, env.EventSchemaVersion,
			env.OccurredAt, env.RecordedAt, env.CorrelationID, causation, env.TraceID)
		if err != nil {
			return false, fmt.Errorf("%w: insert lineage lot: %v", ErrTransient, err)
		}
	case event.EventTypeProductionBatchProduced:
		p, ok := event.AsProductionBatchProduced(env)
		if !ok {
			return false, fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
		}
		res, err = tx.ExecContext(ctx, `
			INSERT INTO platform.lineage_production_batches (
				tenant_id, batch_id, lot_id, aggregate_version,
				source_event_id, event_schema_version,
				occurred_at, recorded_at, correlation_id, causation_id, trace_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT DO NOTHING
		`, env.TenantID, p.BatchID, p.LotID, env.AggregateVersion,
			env.EventID, env.EventSchemaVersion,
			env.OccurredAt, env.RecordedAt, env.CorrelationID, causation, env.TraceID)
		if err != nil {
			return false, fmt.Errorf("%w: insert lineage batch: %v", ErrTransient, err)
		}
	case event.EventTypeShipmentDispatched:
		p, ok := event.AsShipmentDispatched(env)
		if !ok {
			return false, fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
		}
		res, err = tx.ExecContext(ctx, `
			INSERT INTO platform.lineage_shipments (
				tenant_id, shipment_id, batch_id, order_id, aggregate_version,
				source_event_id, event_schema_version,
				occurred_at, recorded_at, correlation_id, causation_id, trace_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT DO NOTHING
		`, env.TenantID, p.ShipmentID, p.BatchID, p.OrderID, env.AggregateVersion,
			env.EventID, env.EventSchemaVersion,
			env.OccurredAt, env.RecordedAt, env.CorrelationID, causation, env.TraceID)
		if err != nil {
			return false, fmt.Errorf("%w: insert lineage shipment: %v", ErrTransient, err)
		}
	default:
		return false, fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("%w: lineage rows affected: %v", ErrTransient, err)
	}
	return n == 1, nil
}

func derivedLineageDeletes() []string {
	return []string{
		`DELETE FROM platform.lineage_shipments`,
		`DELETE FROM platform.lineage_production_batches`,
		`DELETE FROM platform.lineage_ingredient_lots`,
		`DELETE FROM platform.lineage_suppliers`,
	}
}

func derivedLineageDeletesForTenant() []string {
	return []string{
		`DELETE FROM platform.lineage_shipments WHERE tenant_id = $1`,
		`DELETE FROM platform.lineage_production_batches WHERE tenant_id = $1`,
		`DELETE FROM platform.lineage_ingredient_lots WHERE tenant_id = $1`,
		`DELETE FROM platform.lineage_suppliers WHERE tenant_id = $1`,
	}
}

// TraceBatch returns the tenant-scoped lineage chain for batchID.
// Tenant identity comes only from tenantID; a missing batch yields ok=false.
func TraceBatch(ctx context.Context, db *sql.DB, tenantID, batchID string) (BatchTrace, bool, error) {
	if db == nil {
		return BatchTrace{}, false, fmt.Errorf("platform: nil db")
	}
	tenantID = strings.TrimSpace(tenantID)
	batchID = strings.TrimSpace(batchID)
	if tenantID == "" {
		return BatchTrace{}, false, fmt.Errorf("platform: tenant_id required")
	}
	if batchID == "" {
		return BatchTrace{}, false, fmt.Errorf("platform: batch_id required")
	}

	var (
		batch lineageHopScan
		lot   lineageHopScan
		sup   lineageHopScan
		ship  lineageHopScan
	)
	dest := append([]any{}, batch.scanDest()...)
	dest = append(dest, lot.scanDest()...)
	dest = append(dest, sup.scanDest()...)
	dest = append(dest, ship.scanDest()...)
	err := db.QueryRowContext(ctx, `
		SELECT
			b.batch_id, b.lot_id, NULL, NULL, b.aggregate_version, b.source_event_id, b.event_schema_version,
			b.occurred_at, b.recorded_at, b.correlation_id, b.causation_id, b.trace_id,
			l.lot_id, l.supplier_id, l.item_id, NULL, l.aggregate_version, l.source_event_id, l.event_schema_version,
			l.occurred_at, l.recorded_at, l.correlation_id, l.causation_id, l.trace_id,
			s.supplier_id, NULL, NULL, NULL, s.aggregate_version, s.source_event_id, s.event_schema_version,
			s.occurred_at, s.recorded_at, s.correlation_id, s.causation_id, s.trace_id,
			sh.shipment_id, sh.batch_id, NULL, sh.order_id, sh.aggregate_version, sh.source_event_id, sh.event_schema_version,
			sh.occurred_at, sh.recorded_at, sh.correlation_id, sh.causation_id, sh.trace_id
		FROM platform.lineage_production_batches b
		LEFT JOIN platform.lineage_ingredient_lots l
		  ON l.tenant_id = b.tenant_id AND l.lot_id = b.lot_id
		LEFT JOIN platform.lineage_suppliers s
		  ON s.tenant_id = b.tenant_id AND s.supplier_id = l.supplier_id
		LEFT JOIN platform.lineage_shipments sh
		  ON sh.tenant_id = b.tenant_id AND sh.batch_id = b.batch_id
		WHERE b.tenant_id = $1 AND b.batch_id = $2
	`, tenantID, batchID).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return BatchTrace{TenantID: tenantID}, false, nil
	}
	if err != nil {
		return BatchTrace{}, false, fmt.Errorf("platform: trace batch: %w", err)
	}
	return BatchTrace{
		TenantID: tenantID,
		Batch:    batch.hop(),
		Lot:      lot.hop(),
		Supplier: sup.hop(),
		Shipment: ship.hop(),
	}, true, nil
}

type lineageHopScan struct {
	id, parent, item, orderID                           sql.NullString
	version, schema                                     sql.NullInt64
	eventID, occurred, recorded, corr, causation, trace sql.NullString
}

func (h *lineageHopScan) scanDest() []any {
	return []any{
		&h.id, &h.parent, &h.item, &h.orderID, &h.version, &h.eventID, &h.schema,
		&h.occurred, &h.recorded, &h.corr, &h.causation, &h.trace,
	}
}

func (h lineageHopScan) hop() LineageHop {
	out := LineageHop{
		ID:                 h.id.String,
		ParentID:           h.parent.String,
		ItemID:             h.item.String,
		OrderID:            h.orderID.String,
		AggregateVersion:   h.version.Int64,
		SourceEventID:      h.eventID.String,
		EventSchemaVersion: h.schema.Int64,
		OccurredAt:         h.occurred.String,
		RecordedAt:         h.recorded.String,
		CorrelationID:      h.corr.String,
		TraceID:            h.trace.String,
	}
	if h.causation.Valid {
		c := h.causation.String
		out.CausationID = &c
	}
	return out
}

func countLineageRows(ctx context.Context, db *sql.DB, tenantID string) (suppliers, lots, batches, shipments int, err error) {
	scan := func(q string, dest *int) error {
		return db.QueryRowContext(ctx, q, tenantID).Scan(dest)
	}
	if err = scan(`SELECT COUNT(*) FROM platform.lineage_suppliers WHERE tenant_id = $1`, &suppliers); err != nil {
		return
	}
	if err = scan(`SELECT COUNT(*) FROM platform.lineage_ingredient_lots WHERE tenant_id = $1`, &lots); err != nil {
		return
	}
	if err = scan(`SELECT COUNT(*) FROM platform.lineage_production_batches WHERE tenant_id = $1`, &batches); err != nil {
		return
	}
	err = scan(`SELECT COUNT(*) FROM platform.lineage_shipments WHERE tenant_id = $1`, &shipments)
	return
}
