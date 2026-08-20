// Package bootstrap creates the public Northstar Foods scenario through the
// implemented ERP, outbox, relay, and projection boundaries.
package bootstrap

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
)

const (
	// DefaultTimeout bounds the time spent waiting for the real relay and
	// consumer to reach the declared checkpoint.
	DefaultTimeout = 2 * time.Minute
	// DefaultPollTimeout keeps an idle consumer cycle bounded while the
	// bootstrap deadline remains the outer bound.
	DefaultPollTimeout   = 500 * time.Millisecond
	DefaultRetryInterval = 100 * time.Millisecond
	DefaultLeaseTTL      = relay.DefaultLeaseTTL
	DefaultDrainLimit    = 100

	// DemoPrincipalID is the deterministic local operator identity documented
	// for the public Northstar scenario. It is not an OIDC credential.
	DemoPrincipalID = "northstar-demo-operator"
)

var (
	// ErrImmutableFixtureConflict means existing source state does not match
	// the declared fixture bytes and identity.
	ErrImmutableFixtureConflict = errors.New("bootstrap: immutable fixture conflict")
	// ErrCheckpointTimeout means source state was accepted but the relay and
	// consumer did not reach the expected projection checkpoint in time.
	ErrCheckpointTimeout = errors.New("bootstrap: projection checkpoint timeout")
)

// Consumer is the real or test projection consumer used by Run.
type Consumer interface {
	ConsumeOnce(context.Context) (platform.ConsumeResult, error)
}

// Config controls one bounded bootstrap run.
type Config struct {
	Seed          string
	Timeout       time.Duration
	PollTimeout   time.Duration
	RetryInterval time.Duration
	RelayOwner    string
	LeaseTTL      time.Duration
	DrainLimit    int
}

// EventCounts reports the fixture-scoped source and processing evidence.
type EventCounts struct {
	Source        int `json:"source"`
	Published     int `json:"published"`
	Projected     int `json:"projected"`
	DuplicateNoop int `json:"duplicate_noop"`
}

// LineageRootIDs identifies every immutable Northstar lineage node.
type LineageRootIDs struct {
	SupplierID string `json:"supplier_id"`
	LotID      string `json:"lot_id"`
	BatchID    string `json:"batch_id"`
	ShipmentID string `json:"shipment_id"`
	OrderID    string `json:"order_id"`
}

// AuthorizationConfig is the reviewable local demo assignment configuration.
// TENANT-NS-002 intentionally has no assignment or matrix row.
type AuthorizationConfig struct {
	PrincipalID      string                `json:"principal_id"`
	AllowedTenantID  string                `json:"allowed_tenant_id"`
	NegativeTenantID string                `json:"negative_tenant_id"`
	Assignments      []identity.Assignment `json:"assignments"`
}

// Summary is the machine-readable completion record emitted by the command.
// It contains no credentials, cookies, or raw event bytes.
type Summary struct {
	Status             string              `json:"status"`
	FixtureVersion     string              `json:"fixture_version"`
	TenantID           string              `json:"tenant_id"`
	NegativeTenantID   string              `json:"negative_tenant_id"`
	ItemID             string              `json:"item_id"`
	EventIDs           []string            `json:"event_ids"`
	EventCounts        EventCounts         `json:"event_counts"`
	OutboxState        map[string]int      `json:"outbox_state"`
	ProjectionChecksum string              `json:"projection_checksum"`
	LineageChecksum    string              `json:"lineage_checksum"`
	LineageRootIDs     LineageRootIDs      `json:"lineage_root_ids"`
	Authorization      AuthorizationConfig `json:"authorization"`
}

// DemoAssignments returns a fresh copy of the explicit Northstar operator
// assignments. No row is created for the cross-tenant negative case.
func DemoAssignments() []identity.Assignment {
	return []identity.Assignment{
		{PrincipalID: DemoPrincipalID, TenantID: identity.TenantNS001UUID, RoleID: identity.RoleOpsReader},
		{PrincipalID: DemoPrincipalID, TenantID: identity.TenantNS001UUID, RoleID: identity.RolePlatformOperator},
	}
}

// Run accepts the deterministic Northstar lineage fixture through ERP source
// methods, drains the transactional outbox, and waits for the real consumer
// to apply every event. Existing matching source state is reused safely.
func Run(ctx context.Context, db *sql.DB, pub relay.Publisher, consumer Consumer, cfg Config) (Summary, error) {
	if err := validateConfig(db, pub, consumer, cfg); err != nil {
		return Summary{}, err
	}
	fx, err := northstar.GenerateLineage(cfg.Seed)
	if err != nil {
		return Summary{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	if err := ensureInventory(runCtx, db, fx); err != nil {
		return Summary{}, err
	}
	if err := ensureSources(runCtx, db, fx); err != nil {
		return Summary{}, err
	}

	deadline := time.Now().Add(cfg.Timeout)
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		state, complete, err := inspectCheckpoint(runCtx, db, fx)
		if err != nil {
			if errors.Is(err, ErrImmutableFixtureConflict) {
				return Summary{}, err
			}
			lastErr = err
		} else if complete {
			return state, nil
		}
		if remaining := time.Until(deadline); remaining <= 0 {
			if lastErr == nil {
				lastErr = fmt.Errorf("checkpoint incomplete")
			}
			return Summary{}, fmt.Errorf("%w after %s: %v", ErrCheckpointTimeout, cfg.Timeout, lastErr)
		}

		cycleTimeout := cfg.PollTimeout
		if remaining := time.Until(deadline); remaining < cycleTimeout {
			cycleTimeout = remaining
		}
		cycleCtx, cancel := context.WithTimeout(runCtx, cycleTimeout)
		drain, drainErr := relay.DrainOnce(cycleCtx, db, pub, cfg.RelayOwner, cfg.LeaseTTL, cfg.DrainLimit)
		cancel()
		if drainErr != nil {
			lastErr = drainErr
		}
		if drain.Quarantined > 0 {
			return Summary{}, fmt.Errorf("bootstrap: fixture outbox quarantined")
		}

		cycleCtx, cancel = context.WithTimeout(runCtx, cycleTimeout)
		_, consumeErr := consumer.ConsumeOnce(cycleCtx)
		cancel()
		if consumeErr != nil && !errors.Is(consumeErr, context.DeadlineExceeded) && !errors.Is(consumeErr, context.Canceled) {
			lastErr = consumeErr
		}

		if !waitFor(runCtx, cfg.RetryInterval) {
			if err := ctx.Err(); err != nil {
				return Summary{}, err
			}
			return Summary{}, fmt.Errorf("%w after %s: %v", ErrCheckpointTimeout, cfg.Timeout, lastErr)
		}
	}
}

func validateConfig(db *sql.DB, pub relay.Publisher, consumer Consumer, cfg Config) error {
	if db == nil {
		return fmt.Errorf("bootstrap: database required")
	}
	if pub == nil {
		return fmt.Errorf("bootstrap: relay publisher required")
	}
	if consumer == nil {
		return fmt.Errorf("bootstrap: projection consumer required")
	}
	if strings.TrimSpace(cfg.Seed) == "" {
		return fmt.Errorf("bootstrap: fixture seed required")
	}
	if cfg.Timeout <= 0 || cfg.PollTimeout <= 0 || cfg.RetryInterval <= 0 {
		return fmt.Errorf("bootstrap: timeouts and retry interval must be positive")
	}
	if cfg.RelayOwner == "" {
		return fmt.Errorf("bootstrap: relay owner required")
	}
	if cfg.LeaseTTL <= 0 || cfg.DrainLimit < 1 {
		return fmt.Errorf("bootstrap: relay settings are invalid")
	}
	return nil
}

type sourceStep struct {
	env    event.Envelope
	accept func(context.Context, *sql.DB) (erp.SourceAcceptResult, error)
}

func ensureSources(ctx context.Context, db *sql.DB, fx northstar.LineageFixture) error {
	supplier, err := erp.SupplierCommandFromLineage(fx)
	if err != nil {
		return err
	}
	lot, err := erp.IngredientLotCommandFromLineage(fx)
	if err != nil {
		return err
	}
	batch, err := erp.ProductionBatchCommandFromLineage(fx)
	if err != nil {
		return err
	}
	shipment, err := erp.ShipmentCommandFromLineage(fx)
	if err != nil {
		return err
	}
	order, err := erp.OrderCommandFromLineage(fx)
	if err != nil {
		return err
	}
	steps := []sourceStep{
		{env: fx.Events[0], accept: func(ctx context.Context, db *sql.DB) (erp.SourceAcceptResult, error) {
			result, err := erp.RegisterSupplier(ctx, db, supplier)
			return result, err
		}},
		{env: fx.Events[1], accept: func(ctx context.Context, db *sql.DB) (erp.SourceAcceptResult, error) {
			result, err := erp.ReceiveIngredientLot(ctx, db, lot)
			return result, err
		}},
		{env: fx.Events[2], accept: func(ctx context.Context, db *sql.DB) (erp.SourceAcceptResult, error) {
			result, err := erp.ProduceProductionBatch(ctx, db, batch)
			return result, err
		}},
		{env: fx.Events[3], accept: func(ctx context.Context, db *sql.DB) (erp.SourceAcceptResult, error) {
			result, err := erp.DispatchShipment(ctx, db, shipment)
			return result, err
		}},
		{env: fx.Events[4], accept: func(ctx context.Context, db *sql.DB) (erp.SourceAcceptResult, error) {
			result, err := erp.AcceptOrder(ctx, db, order)
			return erp.SourceAcceptResult{
				AggregateID:      result.ItemID,
				AggregateVersion: result.AggregateVersion,
				EventID:          result.EventID,
				ContentHash:      result.ContentHash,
				EventBytes:       result.EventBytes,
				OutboxStatus:     result.OutboxStatus,
			}, err
		}},
	}
	for _, step := range steps {
		if err := ensureSource(ctx, db, step.env, step.accept); err != nil {
			return err
		}
	}
	return nil
}

func ensureSource(ctx context.Context, db *sql.DB, env event.Envelope, accept func(context.Context, *sql.DB) (erp.SourceAcceptResult, error)) error {
	if result, err := accept(ctx, db); err == nil {
		return verifySourceResult(env, result)
	} else if !errors.Is(err, erp.ErrDuplicateSource) && !errors.Is(err, erp.ErrDuplicateOrder) && !errors.Is(err, erp.ErrDuplicateEvent) {
		return err
	} else {
		row, found, lookupErr := loadExpectedOutbox(ctx, db, env)
		if lookupErr != nil {
			return lookupErr
		}
		if !found {
			return fmt.Errorf("%w: existing source has no matching outbox event %s", ErrImmutableFixtureConflict, env.EventID)
		}
		return verifyOutbox(env, row)
	}
}

func verifySourceResult(env event.Envelope, result erp.SourceAcceptResult) error {
	canonical, err := event.CanonicalBytes(env)
	if err != nil {
		return err
	}
	hash, err := event.ContentHash(env)
	if err != nil {
		return err
	}
	if result.EventID != env.EventID || result.ContentHash != hash || !bytes.Equal(result.EventBytes, canonical) {
		return fmt.Errorf("%w: accepted event %s differs from declared fixture", ErrImmutableFixtureConflict, env.EventID)
	}
	return nil
}

type outboxRow struct {
	eventID       string
	tenantID      string
	aggregateType string
	aggregateID   string
	version       int64
	contentHash   string
	eventBytes    []byte
	status        string
}

func loadExpectedOutbox(ctx context.Context, db *sql.DB, env event.Envelope) (outboxRow, bool, error) {
	var row outboxRow
	err := db.QueryRowContext(ctx, `
		SELECT event_id, tenant_id, aggregate_type, aggregate_id,
		       aggregate_version, content_hash, event_bytes, status
		FROM erp.outbox
		WHERE tenant_id = $1 AND aggregate_type = $2 AND aggregate_id = $3
		  AND aggregate_version = $4
	`, env.TenantID, env.AggregateType, env.AggregateID, env.AggregateVersion).Scan(
		&row.eventID, &row.tenantID, &row.aggregateType, &row.aggregateID,
		&row.version, &row.contentHash, &row.eventBytes, &row.status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		var eventID string
		err = db.QueryRowContext(ctx, `SELECT event_id FROM erp.outbox WHERE event_id = $1`, env.EventID).Scan(&eventID)
		if errors.Is(err, sql.ErrNoRows) {
			return outboxRow{}, false, nil
		}
		if err != nil {
			return outboxRow{}, false, fmt.Errorf("bootstrap: inspect conflicting outbox event: %w", err)
		}
		return outboxRow{}, false, fmt.Errorf("%w: event id %s belongs to another aggregate", ErrImmutableFixtureConflict, env.EventID)
	}
	if err != nil {
		return outboxRow{}, false, fmt.Errorf("bootstrap: inspect outbox: %w", err)
	}
	return row, true, nil
}

func verifyOutbox(env event.Envelope, row outboxRow) error {
	canonical, err := event.CanonicalBytes(env)
	if err != nil {
		return err
	}
	hash, err := event.ContentHash(env)
	if err != nil {
		return err
	}
	if row.eventID != env.EventID || row.tenantID != env.TenantID || row.aggregateType != env.AggregateType || row.aggregateID != env.AggregateID || row.version != env.AggregateVersion || row.contentHash != hash || !bytes.Equal(row.eventBytes, canonical) {
		return fmt.Errorf("%w: outbox event %s differs from declared fixture", ErrImmutableFixtureConflict, env.EventID)
	}
	return nil
}

func ensureInventory(ctx context.Context, db *sql.DB, fx northstar.LineageFixture) error {
	if err := erp.SeedLineageInventory(ctx, db, fx); err == nil {
		return nil
	} else {
		var quantity, version int64
		lookupErr := db.QueryRowContext(ctx, `
			SELECT quantity_on_hand, aggregate_version
			FROM erp.inventory_items
			WHERE tenant_id = $1 AND item_id = $2
		`, fx.TenantID, fx.ItemID).Scan(&quantity, &version)
		if lookupErr != nil {
			if errors.Is(lookupErr, sql.ErrNoRows) {
				return err
			}
			return fmt.Errorf("bootstrap: inspect inventory after seed failure: %w", lookupErr)
		}
		// The seed row is valid both before and after the one declared order.
		if quantity == 10 && version == 0 {
			return nil
		}
		if quantity == 8 && version == 1 {
			row, found, inspectErr := loadExpectedOutbox(ctx, db, fx.Events[4])
			if inspectErr != nil {
				return inspectErr
			}
			if !found {
				return fmt.Errorf("%w: final inventory state has no declared order event", ErrImmutableFixtureConflict)
			}
			return verifyOutbox(fx.Events[4], row)
		}
		return fmt.Errorf("%w: inventory %s has quantity=%d version=%d", ErrImmutableFixtureConflict, fx.ItemID, quantity, version)
	}
}

func inspectCheckpoint(ctx context.Context, db *sql.DB, fx northstar.LineageFixture) (Summary, bool, error) {
	summary := Summary{
		Status:           "complete",
		FixtureVersion:   fx.Seed,
		TenantID:         fx.TenantID,
		NegativeTenantID: identity.TenantNS002UUID,
		ItemID:           fx.ItemID,
		EventIDs:         make([]string, 0, len(fx.Events)),
		OutboxState:      map[string]int{},
		LineageRootIDs:   LineageRootIDs{SupplierID: fx.SupplierID, LotID: fx.LotID, BatchID: fx.BatchID, ShipmentID: fx.ShipmentID, OrderID: fx.OrderID},
		Authorization:    AuthorizationConfig{PrincipalID: DemoPrincipalID, AllowedTenantID: identity.TenantNS001UUID, NegativeTenantID: identity.TenantNS002UUID, Assignments: DemoAssignments()},
	}
	for _, env := range fx.Events {
		summary.EventIDs = append(summary.EventIDs, env.EventID)
		row, found, err := loadExpectedOutbox(ctx, db, env)
		if err != nil {
			return Summary{}, false, err
		}
		if !found {
			return summary, false, nil
		}
		if err := verifyOutbox(env, row); err != nil {
			return Summary{}, false, err
		}
		summary.EventCounts.Source++
		summary.OutboxState[row.status]++
		if row.status != relay.StatusPublished {
			continue
		}
		summary.EventCounts.Published++

		var tenantID, contentHash, aggregateType, aggregateID, disposition string
		var version int64
		err = db.QueryRowContext(ctx, `
			SELECT tenant_id, content_hash, aggregate_type, aggregate_id,
			       aggregate_version, disposition
			FROM platform.inbox
			WHERE consumer_name = $1 AND event_id = $2
		`, platform.ConsumerName, env.EventID).Scan(&tenantID, &contentHash, &aggregateType, &aggregateID, &version, &disposition)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return Summary{}, false, fmt.Errorf("bootstrap: inspect inbox: %w", err)
		}
		if tenantID != env.TenantID || contentHash != row.contentHash || aggregateType != env.AggregateType || aggregateID != env.AggregateID || version != env.AggregateVersion {
			return Summary{}, false, fmt.Errorf("%w: inbox event %s differs from declared fixture", ErrImmutableFixtureConflict, env.EventID)
		}
		switch disposition {
		case platform.DispositionApplied:
			summary.EventCounts.Projected++
		case platform.DispositionDuplicateNoop:
			summary.EventCounts.Projected++
			summary.EventCounts.DuplicateNoop++
		case platform.DispositionQuarantinedGap:
			// A gap may be transient while the consumer catches up.
		default:
			return Summary{}, false, fmt.Errorf("bootstrap: fixture event %s reached terminal inbox disposition %s", env.EventID, disposition)
		}
	}
	if summary.EventCounts.Published != len(fx.Events) || summary.EventCounts.Projected != len(fx.Events) {
		return summary, false, nil
	}

	var quantity, version int64
	err := db.QueryRowContext(ctx, `
		SELECT quantity_on_hand, aggregate_version
		FROM platform.inventory_projection
		WHERE tenant_id = $1 AND item_id = $2
	`, fx.TenantID, fx.ItemID).Scan(&quantity, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return summary, false, nil
	}
	if err != nil {
		return Summary{}, false, fmt.Errorf("bootstrap: inspect inventory projection: %w", err)
	}
	if quantity != 8 || version != 1 {
		return Summary{}, false, fmt.Errorf("%w: inventory projection has quantity=%d version=%d", ErrImmutableFixtureConflict, quantity, version)
	}

	lineageChecks := []struct {
		table string
		id    string
		event string
	}{
		{table: "platform.lineage_suppliers", id: fx.SupplierID, event: fx.Events[0].EventID},
		{table: "platform.lineage_ingredient_lots", id: fx.LotID, event: fx.Events[1].EventID},
		{table: "platform.lineage_production_batches", id: fx.BatchID, event: fx.Events[2].EventID},
		{table: "platform.lineage_shipments", id: fx.ShipmentID, event: fx.Events[3].EventID},
	}
	for _, check := range lineageChecks {
		var sourceEvent string
		query := fmt.Sprintf("SELECT source_event_id FROM %s WHERE tenant_id = $1 AND %s = $2", check.table, lineageIDColumn(check.table))
		err := db.QueryRowContext(ctx, query, fx.TenantID, check.id).Scan(&sourceEvent)
		if errors.Is(err, sql.ErrNoRows) {
			return summary, false, nil
		}
		if err != nil {
			return Summary{}, false, fmt.Errorf("bootstrap: inspect lineage: %w", err)
		}
		if sourceEvent != check.event {
			return Summary{}, false, fmt.Errorf("%w: lineage %s has source event %s", ErrImmutableFixtureConflict, check.id, sourceEvent)
		}
	}

	var errChecksum error
	summary.ProjectionChecksum, errChecksum = platform.ChecksumTenant(ctx, db, fx.TenantID)
	if errChecksum != nil {
		return Summary{}, false, errChecksum
	}
	summary.LineageChecksum, errChecksum = platform.ChecksumLineage(ctx, db, fx.TenantID)
	if errChecksum != nil {
		return Summary{}, false, errChecksum
	}
	return summary, true, nil
}

func lineageIDColumn(table string) string {
	switch table {
	case "platform.lineage_suppliers":
		return "supplier_id"
	case "platform.lineage_ingredient_lots":
		return "lot_id"
	case "platform.lineage_production_batches":
		return "batch_id"
	case "platform.lineage_shipments":
		return "shipment_id"
	default:
		panic("bootstrap: unknown lineage table")
	}
}

func waitFor(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// MarshalSummary is kept as a small command-facing helper so output remains
// stable and machine-readable across callers.
func MarshalSummary(summary Summary) ([]byte, error) {
	return json.Marshal(summary)
}
