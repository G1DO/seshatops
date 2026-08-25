package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/G1DO/seshatops/bootstrap"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/relay"
)

const (
	envDemoFixtureConfirm   = "SESHATOPS_DEMO_FIXTURE_CONFIRM"
	demoFixtureConfirmation = "I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO"
	demoFixtureBroker       = "redpanda:9092"

	demoPoisonFixtureVersion             = "northstar-m5-poison-v1"
	demoPoisonEventID                    = "318f5d78-6e64-4f5f-bd16-8e9f7c4a4011"
	demoPoisonCorrelationID              = "318f5d78-6e64-4f5f-bd16-8e9f7c4a4001"
	demoPoisonSupplierID                 = "mill-northstar-poison-001"
	demoForecastIncompleteFixtureVersion = "northstar-m5-forecast-incomplete-v1"
	demoForecastIncompleteEventID        = "318f5d78-6e64-4f5f-bd16-8e9f7c4a4021"
	demoForecastIncompleteItemID         = "item-flour-forecast-incomplete-001"
	demoFaultRecordedAt                  = "2026-08-07T10:05:00Z"
)

type demoFixtureAction string

const (
	demoFixtureSource             demoFixtureAction = "source"
	demoFixtureInspect            demoFixtureAction = "inspect"
	demoFixtureDuplicate          demoFixtureAction = "duplicate"
	demoFixturePoison             demoFixtureAction = "poison"
	demoFixtureForecastIncomplete demoFixtureAction = "forecast-incomplete"
)

type demoFixtureCommandConfig struct {
	Action      demoFixtureAction
	DatabaseURL string
	BrokerSeeds []string
	Timeout     time.Duration
}

type demoFixtureSummary struct {
	Status             string                 `json:"status"`
	Action             string                 `json:"action"`
	FixtureVersion     string                 `json:"fixture_version"`
	TenantID           string                 `json:"tenant_id"`
	EventID            string                 `json:"event_id,omitempty"`
	SourceCount        int                    `json:"source_count,omitempty"`
	OutboxState        map[string]int         `json:"outbox_state,omitempty"`
	OutboxStatus       string                 `json:"outbox_status,omitempty"`
	EventCounts        *bootstrap.EventCounts `json:"event_counts,omitempty"`
	ProjectionChecksum string                 `json:"projection_checksum,omitempty"`
	LineageChecksum    string                 `json:"lineage_checksum,omitempty"`
	Topic              string                 `json:"topic,omitempty"`
	Key                string                 `json:"key,omitempty"`
}

type demoBrokerRecord struct {
	FixtureVersion string
	EventID        string
	TenantID       string
	Topic          string
	Key            []byte
	Value          []byte
	OutboxStatus   string
}

type demoOutboxRecord struct {
	FixtureVersion   string
	EventID          string
	TenantID         string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	ContentHash      string
	EventBytes       []byte
	Status           string
	RecordedAt       time.Time
	LastErrorCode    string
}

func loadDemoFixtureCommandConfig(args []string) (demoFixtureCommandConfig, error) {
	return demoFixtureCommandConfigFromEnv(args, os.LookupEnv)
}

func demoFixtureCommandConfigFromEnv(args []string, lookup func(string) (string, bool)) (demoFixtureCommandConfig, error) {
	action, err := parseDemoFixtureAction(args)
	if err != nil {
		return demoFixtureCommandConfig{}, err
	}
	localStack, ok := lookup(envLocalStack)
	if !ok || strings.TrimSpace(localStack) != "true" {
		return demoFixtureCommandConfig{}, fmt.Errorf("%s must equal true", envLocalStack)
	}
	confirmation, ok := lookup(envDemoFixtureConfirm)
	if !ok || strings.TrimSpace(confirmation) != demoFixtureConfirmation {
		return demoFixtureCommandConfig{}, fmt.Errorf("%s must equal %s", envDemoFixtureConfirm, demoFixtureConfirmation)
	}
	databaseURL, err := requiredEnv(lookup, envDatabaseURL)
	if err != nil {
		return demoFixtureCommandConfig{}, err
	}
	if err := validateInternalDemoDatabaseURL(databaseURL); err != nil {
		return demoFixtureCommandConfig{}, fmt.Errorf("%s: %w", envDatabaseURL, err)
	}

	cfg := demoFixtureCommandConfig{
		Action:      action,
		DatabaseURL: databaseURL,
		Timeout:     defaultStartup,
	}
	if action.needsBroker() {
		rawSeeds, err := requiredEnv(lookup, envBrokerSeeds)
		if err != nil {
			return demoFixtureCommandConfig{}, err
		}
		seeds, err := parseBrokerSeeds(rawSeeds)
		if err != nil {
			return demoFixtureCommandConfig{}, fmt.Errorf("%s: %w", envBrokerSeeds, err)
		}
		if len(seeds) != 1 || seeds[0] != demoFixtureBroker {
			return demoFixtureCommandConfig{}, fmt.Errorf("%s must equal %s", envBrokerSeeds, demoFixtureBroker)
		}
		cfg.BrokerSeeds = seeds
	}
	return cfg, nil
}

func parseDemoFixtureAction(args []string) (demoFixtureAction, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("demo-fixture requires exactly one action")
	}
	action := demoFixtureAction(args[0])
	switch action {
	case demoFixtureSource, demoFixtureInspect, demoFixtureDuplicate, demoFixturePoison, demoFixtureForecastIncomplete:
		return action, nil
	default:
		return "", fmt.Errorf("unsupported demo-fixture action %q", args[0])
	}
}

func (a demoFixtureAction) needsBroker() bool {
	return a == demoFixtureDuplicate || a == demoFixturePoison
}

func validateInternalDemoDatabaseURL(raw string) error {
	if err := validateDatabaseURL(raw); err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("must target the internal disposable database")
	}
	if strings.ToLower(u.Hostname()) != "postgres" {
		return fmt.Errorf("database host must be postgres")
	}
	if u.User == nil || u.User.Username() != "seshatops" {
		return fmt.Errorf("database user must be seshatops")
	}
	if strings.TrimPrefix(u.Path, "/") != northstarDisposableDatabase {
		return fmt.Errorf("database name must be %s", northstarDisposableDatabase)
	}
	port := u.Port()
	if port != "5432" {
		return fmt.Errorf("database port must be 5432")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return fmt.Errorf("database query parameters are invalid")
	}
	// Connection-target parameters such as host, hostaddr, dbname, and service
	// could override the reviewed authority/path. Require the exact transport
	// setting from the packaged disposable stack instead.
	if len(query) != 1 || len(query["sslmode"]) != 1 || query["sslmode"][0] != "disable" {
		return fmt.Errorf("database query must equal sslmode=disable")
	}
	return nil
}

func runDemoFixtureCommand(ctx context.Context, cfg demoFixtureCommandConfig, out io.Writer) error {
	if cfg.Timeout <= 0 {
		return fmt.Errorf("demo-fixture timeout must be positive")
	}
	commandCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	var (
		summary demoFixtureSummary
		err     error
	)
	switch cfg.Action {
	case demoFixtureSource:
		summary, err = runDemoSourceFixture(commandCtx, cfg.DatabaseURL)
	case demoFixtureInspect:
		summary, err = runDemoInspectFixture(commandCtx, cfg.DatabaseURL)
	case demoFixtureDuplicate:
		summary, err = runDemoDuplicateFixture(commandCtx, cfg.DatabaseURL, cfg.BrokerSeeds)
	case demoFixturePoison:
		summary, err = runDemoPoisonFixture(commandCtx, cfg.BrokerSeeds)
	case demoFixtureForecastIncomplete:
		summary, err = runDemoForecastIncompleteFixture(commandCtx, cfg.DatabaseURL)
	default:
		return fmt.Errorf("unsupported demo-fixture action %q", cfg.Action)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(summary)
}

func runDemoInspectFixture(ctx context.Context, databaseURL string) (demoFixtureSummary, error) {
	db, err := openDemoFixtureDatabase(ctx, databaseURL)
	if err != nil {
		return demoFixtureSummary{}, err
	}
	defer db.Close()

	checkpoint, complete, err := bootstrap.InspectFixtureCheckpoint(ctx, db, northstar.LineageSeed)
	if err != nil {
		return demoFixtureSummary{}, err
	}
	if !complete {
		return demoFixtureSummary{}, fmt.Errorf("demo-fixture: deterministic checkpoint is incomplete")
	}
	return demoFixtureSummary{
		Status:             "complete",
		Action:             string(demoFixtureInspect),
		FixtureVersion:     checkpoint.FixtureVersion,
		TenantID:           checkpoint.TenantID,
		SourceCount:        checkpoint.EventCounts.Source,
		OutboxState:        checkpoint.OutboxState,
		EventCounts:        &checkpoint.EventCounts,
		ProjectionChecksum: checkpoint.ProjectionChecksum,
		LineageChecksum:    checkpoint.LineageChecksum,
	}, nil
}

func runDemoSourceFixture(ctx context.Context, databaseURL string) (demoFixtureSummary, error) {
	db, err := openDemoFixtureDatabase(ctx, databaseURL)
	if err != nil {
		return demoFixtureSummary{}, err
	}
	defer db.Close()

	source, err := bootstrap.AcceptSourceFixture(ctx, db, northstar.LineageSeed)
	if err != nil {
		return demoFixtureSummary{}, err
	}
	var eventID string
	if len(source.EventIDs) != 0 {
		eventID = source.EventIDs[len(source.EventIDs)-1]
	}
	return demoFixtureSummary{
		Status:         "complete",
		Action:         string(demoFixtureSource),
		FixtureVersion: source.FixtureVersion,
		TenantID:       source.TenantID,
		EventID:        eventID,
		SourceCount:    source.SourceCount,
		OutboxState:    source.OutboxState,
	}, nil
}

func runDemoDuplicateFixture(ctx context.Context, databaseURL string, seeds []string) (demoFixtureSummary, error) {
	db, err := openDemoFixtureDatabase(ctx, databaseURL)
	if err != nil {
		return demoFixtureSummary{}, err
	}
	defer db.Close()
	record, err := loadDemoDuplicateRecord(ctx, db)
	if err != nil {
		return demoFixtureSummary{}, err
	}
	if err := publishDemoBrokerRecord(ctx, seeds, record); err != nil {
		return demoFixtureSummary{}, err
	}
	return demoSummaryForBrokerRecord(demoFixtureDuplicate, record), nil
}

func runDemoPoisonFixture(ctx context.Context, seeds []string) (demoFixtureSummary, error) {
	record, err := buildDemoPoisonRecord()
	if err != nil {
		return demoFixtureSummary{}, err
	}
	if err := publishDemoBrokerRecord(ctx, seeds, record); err != nil {
		return demoFixtureSummary{}, err
	}
	return demoSummaryForBrokerRecord(demoFixturePoison, record), nil
}

func runDemoForecastIncompleteFixture(ctx context.Context, databaseURL string) (demoFixtureSummary, error) {
	db, err := openDemoFixtureDatabase(ctx, databaseURL)
	if err != nil {
		return demoFixtureSummary{}, err
	}
	defer db.Close()
	record, err := buildDemoForecastIncompleteRecord()
	if err != nil {
		return demoFixtureSummary{}, err
	}
	if err := insertDemoForecastIncompleteRecord(ctx, db, record); err != nil {
		return demoFixtureSummary{}, err
	}
	return demoFixtureSummary{
		Status:         "complete",
		Action:         string(demoFixtureForecastIncomplete),
		FixtureVersion: record.FixtureVersion,
		TenantID:       record.TenantID,
		EventID:        record.EventID,
		OutboxStatus:   record.Status,
	}, nil
}

func openDemoFixtureDatabase(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database failed; check %s", envDatabaseURL)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database ping failed; check %s", envDatabaseURL)
	}
	return db, nil
}

func loadDemoDuplicateRecord(ctx context.Context, db *sql.DB) (demoBrokerRecord, error) {
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		return demoBrokerRecord{}, err
	}
	env := fx.Events[len(fx.Events)-1]
	wantBytes, err := event.CanonicalBytes(env)
	if err != nil {
		return demoBrokerRecord{}, err
	}
	wantHash, err := event.ContentHash(env)
	if err != nil {
		return demoBrokerRecord{}, err
	}
	var (
		tenantID, aggregateType, aggregateID, contentHash, status string
		aggregateVersion                                          int64
		eventBytes                                                []byte
	)
	err = db.QueryRowContext(ctx, `
		SELECT tenant_id, aggregate_type, aggregate_id, aggregate_version,
		       content_hash, event_bytes, status
		FROM erp.outbox
		WHERE event_id = $1
	`, env.EventID).Scan(&tenantID, &aggregateType, &aggregateID, &aggregateVersion, &contentHash, &eventBytes, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return demoBrokerRecord{}, fmt.Errorf("demo-fixture: final inventory event is not retained")
	}
	if err != nil {
		return demoBrokerRecord{}, fmt.Errorf("demo-fixture: inspect final inventory event: %w", err)
	}
	if tenantID != env.TenantID || aggregateType != env.AggregateType || aggregateID != env.AggregateID || aggregateVersion != env.AggregateVersion || contentHash != wantHash || !bytes.Equal(eventBytes, wantBytes) {
		return demoBrokerRecord{}, fmt.Errorf("demo-fixture: retained final inventory event differs from %s", northstar.LineageSeed)
	}
	if status != relay.StatusPublished {
		return demoBrokerRecord{}, fmt.Errorf("demo-fixture: final inventory event must be published before duplicate delivery")
	}
	return demoBrokerRecord{
		FixtureVersion: fx.Seed,
		EventID:        env.EventID,
		TenantID:       env.TenantID,
		Topic:          relay.Topic,
		Key:            []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID)),
		Value:          append([]byte(nil), eventBytes...),
		OutboxStatus:   status,
	}, nil
}

func buildDemoPoisonRecord() (demoBrokerRecord, error) {
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		return demoBrokerRecord{}, err
	}
	env := fx.Events[0]
	env.EventID = demoPoisonEventID
	env.AggregateID = demoPoisonSupplierID
	env.OccurredAt = demoFaultRecordedAt
	env.RecordedAt = demoFaultRecordedAt
	env.CorrelationID = demoPoisonCorrelationID
	env.CausationID = nil
	env.Payload = event.SupplierRegistered{SupplierID: demoPoisonSupplierID}
	raw, err := event.CanonicalBytes(env)
	if err != nil {
		return demoBrokerRecord{}, fmt.Errorf("demo-fixture: build poison event: %w", err)
	}
	unsupported := bytes.Replace(raw, []byte(`"event_schema_version":1`), []byte(`"event_schema_version":2`), 1)
	if bytes.Equal(unsupported, raw) {
		return demoBrokerRecord{}, fmt.Errorf("demo-fixture: build poison event: schema marker missing")
	}
	if _, err := event.Parse(unsupported); !errors.Is(err, event.ErrUnsupported) {
		return demoBrokerRecord{}, fmt.Errorf("demo-fixture: poison event is not deterministically unsupported")
	}
	return demoBrokerRecord{
		FixtureVersion: demoPoisonFixtureVersion,
		EventID:        env.EventID,
		TenantID:       env.TenantID,
		Topic:          relay.Topic,
		Key:            []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID)),
		Value:          unsupported,
	}, nil
}

func buildDemoForecastIncompleteRecord() (demoOutboxRecord, error) {
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		return demoOutboxRecord{}, err
	}
	recordedAt, err := time.Parse(time.RFC3339, demoFaultRecordedAt)
	if err != nil {
		return demoOutboxRecord{}, err
	}
	raw := []byte(`{"fixture_version":"` + demoForecastIncompleteFixtureVersion + `"}`)
	digest := sha256.Sum256(raw)
	return demoOutboxRecord{
		FixtureVersion:   demoForecastIncompleteFixtureVersion,
		EventID:          demoForecastIncompleteEventID,
		TenantID:         fx.TenantID,
		AggregateType:    event.AggregateTypeInventoryItem,
		AggregateID:      demoForecastIncompleteItemID,
		AggregateVersion: 1,
		ContentHash:      hex.EncodeToString(digest[:]),
		EventBytes:       raw,
		Status:           relay.StatusQuarantined,
		RecordedAt:       recordedAt.UTC(),
		LastErrorCode:    "demo_forecast_incomplete",
	}, nil
}

func insertDemoForecastIncompleteRecord(ctx context.Context, db *sql.DB, want demoOutboxRecord) error {
	if db == nil {
		return fmt.Errorf("demo-fixture: database required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("demo-fixture: begin forecast-incomplete insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO erp.outbox (
			event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at, last_error_code
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT DO NOTHING
	`, want.EventID, want.TenantID, want.AggregateType, want.AggregateID, want.AggregateVersion,
		want.ContentHash, want.EventBytes, want.Status, want.RecordedAt, want.LastErrorCode); err != nil {
		return fmt.Errorf("demo-fixture: insert forecast-incomplete outbox: %w", err)
	}

	var got demoOutboxRecord
	err = tx.QueryRowContext(ctx, `
		SELECT event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
		       content_hash, event_bytes, status, recorded_at, last_error_code
		FROM erp.outbox
		WHERE event_id = $1
	`, want.EventID).Scan(
		&got.EventID, &got.TenantID, &got.AggregateType, &got.AggregateID, &got.AggregateVersion,
		&got.ContentHash, &got.EventBytes, &got.Status, &got.RecordedAt, &got.LastErrorCode,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("demo-fixture: forecast-incomplete identity conflicts with retained outbox state")
	}
	if err != nil {
		return fmt.Errorf("demo-fixture: inspect forecast-incomplete outbox: %w", err)
	}
	if got.EventID != want.EventID || got.TenantID != want.TenantID || got.AggregateType != want.AggregateType || got.AggregateID != want.AggregateID || got.AggregateVersion != want.AggregateVersion || got.ContentHash != want.ContentHash || !bytes.Equal(got.EventBytes, want.EventBytes) || got.Status != want.Status || !got.RecordedAt.Equal(want.RecordedAt) || got.LastErrorCode != want.LastErrorCode {
		return fmt.Errorf("demo-fixture: retained forecast-incomplete outbox record differs from fixture")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("demo-fixture: commit forecast-incomplete outbox: %w", err)
	}
	return nil
}

func publishDemoBrokerRecord(ctx context.Context, seeds []string, record demoBrokerRecord) error {
	publisher, err := relay.NewFranzPublisher(seeds...)
	if err != nil {
		return fmt.Errorf("demo-fixture: create broker publisher: %w", err)
	}
	defer publisher.Close()
	if err := publisher.Ping(ctx); err != nil {
		return fmt.Errorf("broker ping failed; check %s", envBrokerSeeds)
	}
	return publishDemoRecord(ctx, publisher, record)
}

func publishDemoRecord(ctx context.Context, publisher relay.Publisher, record demoBrokerRecord) error {
	if publisher == nil {
		return fmt.Errorf("demo-fixture: publisher required")
	}
	if record.Topic != relay.Topic || len(record.Key) == 0 || len(record.Value) == 0 {
		return fmt.Errorf("demo-fixture: broker record is incomplete")
	}
	if err := publisher.Publish(ctx, record.Topic, record.Key, record.Value); err != nil {
		return fmt.Errorf("demo-fixture: publish %s: %w", record.EventID, err)
	}
	return nil
}

func demoSummaryForBrokerRecord(action demoFixtureAction, record demoBrokerRecord) demoFixtureSummary {
	return demoFixtureSummary{
		Status:         "complete",
		Action:         string(action),
		FixtureVersion: record.FixtureVersion,
		TenantID:       record.TenantID,
		EventID:        record.EventID,
		OutboxStatus:   record.OutboxStatus,
		Topic:          record.Topic,
		Key:            string(record.Key),
	}
}
