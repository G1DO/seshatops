package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/relay"
)

// ConsumerName is the inbox consumer identity and Redpanda consumer group.
const ConsumerName = "seshatops-m1-inventory-projection"

// ConsumerGroup is the Event Spine inventory-projection consumer group (docs/design/specifications/event-spine.md §5).
const ConsumerGroup = ConsumerName

// MaxHandlerAttempts is the poison threshold before durable failure + ack.
const MaxHandlerAttempts = 5

// Disposition values for platform.inbox.
const (
	DispositionApplied               = "applied"
	DispositionDuplicateNoop         = "duplicate_noop"
	DispositionQuarantinedConflict   = "quarantined_conflict"
	DispositionQuarantinedGap        = "quarantined_gap"
	DispositionQuarantinedStale      = "quarantined_stale"
	DispositionQuarantinedInvalid    = "quarantined_invalid"
	DispositionQuarantinedMismatch   = "quarantined_mismatch"
	DispositionQuarantinedTransition = "quarantined_transition"
)

// SourcePosition identifies a Redpanda record for failure attribution.
type SourcePosition struct {
	Topic     string
	Partition int32
	Offset    int64
}

// Result is the durable processing decision for one delivered record.
type Result struct {
	Disposition string
	EventID     string
	ShouldAck   bool
}

// testFailBeforeCommit, when set by same-package tests, runs after all writes
// and before Commit. A non-nil error forces Rollback.
var testFailBeforeCommit func(ctx context.Context) error

func setTestFailBeforeCommitForTest(fn func(ctx context.Context) error) {
	testFailBeforeCommit = fn
}

// testForceHandlerPoison, when set by same-package tests, injects an explicit
// handler-poison fault after parse/key checks and before processValidated.
var testForceHandlerPoison func() error

func setTestForceHandlerPoisonForTest(fn func() error) {
	testForceHandlerPoison = fn
}

// ProcessRecord validates one Redpanda delivery, commits inbox + projection (or
// quarantine) atomically, and reports whether the caller may acknowledge.
func ProcessRecord(ctx context.Context, db *sql.DB, key, value []byte, pos SourcePosition) (Result, error) {
	if db == nil {
		return Result{}, fmt.Errorf("%w: nil db", ErrTransient)
	}
	if pos.Topic == "" {
		pos.Topic = relay.Topic
	}

	env, err := event.Parse(value)
	if err != nil {
		return persistParseFailure(ctx, db, value, pos, err)
	}

	if !isProjectionFamily(env.EventType) {
		return persistConsumerUnsupported(ctx, db, env, value, pos)
	}
	selfID, ok := projectionSelfID(env)
	if !ok {
		return persistParseFailure(ctx, db, value, pos, fmt.Errorf("%w: payload type mismatch", event.ErrMalformed))
	}

	expectedKey := []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID))
	if !bytes.Equal(key, expectedKey) || selfID != env.AggregateID {
		return quarantineInbox(ctx, db, env, value, DispositionQuarantinedMismatch, nil, nil)
	}

	if testForceHandlerPoison != nil {
		if perr := testForceHandlerPoison(); perr != nil {
			return bumpPoisonAttempt(ctx, db, &env, value, pos, "handler_poison", fmt.Errorf("%w: %v", ErrHandlerPoison, perr))
		}
	}

	res, err := processValidated(ctx, db, env, value)
	if err != nil {
		// Explicit handler poison consumes the attempt budget. Retryable
		// processing/DB failures (including begin/commit) must not.
		if errors.Is(err, ErrHandlerPoison) {
			return bumpPoisonAttempt(ctx, db, &env, value, pos, "handler_poison", err)
		}
		if errors.Is(err, ErrTransient) || isRetryableDB(err) {
			return Result{ShouldAck: false}, err
		}
		return Result{}, err
	}

	if res.Disposition == DispositionApplied || res.Disposition == DispositionDuplicateNoop {
		if rerr := redriveGaps(ctx, db, env.TenantID, env.AggregateType, env.AggregateID); rerr != nil {
			// Inbox/projection decision is already durable. Withhold ack so
			// redelivery (duplicate_noop) retries gap re-drive.
			return Result{
				Disposition: res.Disposition,
				EventID:     res.EventID,
				ShouldAck:   false,
			}, fmt.Errorf("%w: gap redrive: %v", ErrTransient, rerr)
		}
	}
	return res, nil
}

func processValidated(ctx context.Context, db *sql.DB, env event.Envelope, raw []byte) (Result, error) {
	hash, err := event.ContentHash(env)
	if err != nil {
		return Result{}, fmt.Errorf("%w: content hash: %v", ErrTransient, err)
	}
	canonical, err := event.CanonicalBytes(env)
	if err != nil {
		return Result{}, fmt.Errorf("%w: canonical bytes: %v", ErrTransient, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("%w: begin: %v", ErrTransient, err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, ok, err := lockInbox(ctx, tx, env.EventID)
	if err != nil {
		return Result{}, err
	}
	if ok {
		if existing.ContentHash != hash {
			if err := upsertInbox(ctx, tx, inboxRow{
				EventID:          env.EventID,
				TenantID:         env.TenantID,
				ContentHash:      existing.ContentHash,
				AggregateType:    env.AggregateType,
				AggregateID:      env.AggregateID,
				AggregateVersion: env.AggregateVersion,
				Disposition:      DispositionQuarantinedConflict,
			}); err != nil {
				return Result{}, err
			}
			if err := commitWithHook(ctx, tx); err != nil {
				return Result{}, err
			}
			return Result{
				Disposition: DispositionQuarantinedConflict,
				EventID:     env.EventID,
				ShouldAck:   true,
			}, nil
		}
		switch existing.Disposition {
		case DispositionApplied, DispositionDuplicateNoop:
			if err := upsertInbox(ctx, tx, inboxRow{
				EventID:          env.EventID,
				TenantID:         env.TenantID,
				ContentHash:      hash,
				AggregateType:    env.AggregateType,
				AggregateID:      env.AggregateID,
				AggregateVersion: env.AggregateVersion,
				Disposition:      DispositionDuplicateNoop,
			}); err != nil {
				return Result{}, err
			}
			if err := commitWithHook(ctx, tx); err != nil {
				return Result{}, err
			}
			return Result{
				Disposition: DispositionDuplicateNoop,
				EventID:     env.EventID,
				ShouldAck:   true,
			}, nil
		case DispositionQuarantinedGap:
			// Matching gap content remains eligible for apply/re-drive below.
		default:
			// Terminal quarantine dispositions: durable no-op ack.
			if err := commitWithHook(ctx, tx); err != nil {
				return Result{}, err
			}
			return Result{
				Disposition: existing.Disposition,
				EventID:     env.EventID,
				ShouldAck:   true,
			}, nil
		}
	}

	disp, err := applyEvent(ctx, tx, env, hash, canonical)
	if err != nil {
		return Result{}, err
	}
	if err := commitWithHook(ctx, tx); err != nil {
		return Result{}, err
	}
	if disp == DispositionApplied && env.EventType == event.EventTypeQuantityDecremented {
		payload, ok := event.AsQuantityDecremented(env)
		if !ok {
			return Result{}, fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
		}
		notifyApplied(AppliedUpdate{
			TenantID:         env.TenantID,
			ItemID:           env.AggregateID,
			QuantityOnHand:   payload.QuantityAfter,
			AggregateVersion: env.AggregateVersion,
			EventID:          env.EventID,
		})
	}
	return Result{
		Disposition: disp,
		EventID:     env.EventID,
		ShouldAck:   true,
	}, nil
}

func applyProjection(ctx context.Context, tx *sql.Tx, env event.Envelope, hash string, canonical []byte) (string, error) {
	payload, ok := event.AsQuantityDecremented(env)
	if !ok {
		return "", fmt.Errorf("%w: payload type mismatch", event.ErrMalformed)
	}
	var (
		qty     int64
		version int64
	)
	err := tx.QueryRowContext(ctx, `
		SELECT quantity_on_hand, aggregate_version
		FROM platform.inventory_projection
		WHERE tenant_id = $1 AND item_id = $2
		FOR UPDATE
	`, env.TenantID, env.AggregateID).Scan(&qty, &version)
	if errors.Is(err, sql.ErrNoRows) {
		if env.AggregateVersion != 1 {
			expected := int64(1)
			received := env.AggregateVersion
			if err := upsertInbox(ctx, tx, inboxRow{
				EventID:          env.EventID,
				TenantID:         env.TenantID,
				ContentHash:      hash,
				AggregateType:    env.AggregateType,
				AggregateID:      env.AggregateID,
				AggregateVersion: env.AggregateVersion,
				Disposition:      DispositionQuarantinedGap,
				ExpectedVersion:  &expected,
				ReceivedVersion:  &received,
				EventBytes:       canonical,
			}); err != nil {
				return "", err
			}
			return DispositionQuarantinedGap, nil
		}
		if err := insertProjection(ctx, tx, env.TenantID, env.AggregateID, payload.QuantityAfter, 1); err != nil {
			return "", err
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
	if err != nil {
		return "", fmt.Errorf("%w: lock projection: %v", ErrTransient, err)
	}

	if env.AggregateVersion <= version {
		disp := DispositionQuarantinedStale
		if env.AggregateVersion == version {
			// Same version with different event identity/content path reaches here
			// only for a new event_id at an already-applied version.
			disp = DispositionQuarantinedStale
		}
		if err := upsertInbox(ctx, tx, inboxRow{
			EventID:          env.EventID,
			TenantID:         env.TenantID,
			ContentHash:      hash,
			AggregateType:    env.AggregateType,
			AggregateID:      env.AggregateID,
			AggregateVersion: env.AggregateVersion,
			Disposition:      disp,
			ExpectedVersion:  ptrInt64(version + 1),
			ReceivedVersion:  ptrInt64(env.AggregateVersion),
		}); err != nil {
			return "", err
		}
		return disp, nil
	}
	if env.AggregateVersion > version+1 {
		if err := upsertInbox(ctx, tx, inboxRow{
			EventID:          env.EventID,
			TenantID:         env.TenantID,
			ContentHash:      hash,
			AggregateType:    env.AggregateType,
			AggregateID:      env.AggregateID,
			AggregateVersion: env.AggregateVersion,
			Disposition:      DispositionQuarantinedGap,
			ExpectedVersion:  ptrInt64(version + 1),
			ReceivedVersion:  ptrInt64(env.AggregateVersion),
			EventBytes:       canonical,
		}); err != nil {
			return "", err
		}
		return DispositionQuarantinedGap, nil
	}

	// Contiguous next version.
	if payload.QuantityBefore != qty {
		if err := upsertInbox(ctx, tx, inboxRow{
			EventID:          env.EventID,
			TenantID:         env.TenantID,
			ContentHash:      hash,
			AggregateType:    env.AggregateType,
			AggregateID:      env.AggregateID,
			AggregateVersion: env.AggregateVersion,
			Disposition:      DispositionQuarantinedTransition,
		}); err != nil {
			return "", err
		}
		return DispositionQuarantinedTransition, nil
	}
	if payload.QuantityBefore-payload.QuantityDecremented != payload.QuantityAfter {
		if err := upsertInbox(ctx, tx, inboxRow{
			EventID:          env.EventID,
			TenantID:         env.TenantID,
			ContentHash:      hash,
			AggregateType:    env.AggregateType,
			AggregateID:      env.AggregateID,
			AggregateVersion: env.AggregateVersion,
			Disposition:      DispositionQuarantinedInvalid,
		}); err != nil {
			return "", err
		}
		return DispositionQuarantinedInvalid, nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE platform.inventory_projection
		SET quantity_on_hand = $1, aggregate_version = $2
		WHERE tenant_id = $3 AND item_id = $4
	`, payload.QuantityAfter, env.AggregateVersion, env.TenantID, env.AggregateID); err != nil {
		return "", fmt.Errorf("%w: update projection: %v", ErrTransient, err)
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

func insertProjection(ctx context.Context, tx *sql.Tx, tenantID, itemID string, qty, version int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO platform.inventory_projection (
			tenant_id, item_id, quantity_on_hand, aggregate_version
		) VALUES ($1, $2, $3, $4)
	`, tenantID, itemID, qty, version)
	if err != nil {
		return fmt.Errorf("%w: insert projection: %v", ErrTransient, err)
	}
	return nil
}

type inboxRow struct {
	EventID          string
	TenantID         string
	ContentHash      string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	Disposition      string
	ExpectedVersion  *int64
	ReceivedVersion  *int64
	EventBytes       []byte
}

func lockInbox(ctx context.Context, tx *sql.Tx, eventID string) (inboxRow, bool, error) {
	var row inboxRow
	var expected, received sql.NullInt64
	var eventBytes []byte
	err := tx.QueryRowContext(ctx, `
		SELECT event_id, tenant_id, content_hash, aggregate_type, aggregate_id,
		       aggregate_version, disposition, expected_version, received_version, event_bytes
		FROM platform.inbox
		WHERE consumer_name = $1 AND event_id = $2
		FOR UPDATE
	`, ConsumerName, eventID).Scan(
		&row.EventID, &row.TenantID, &row.ContentHash, &row.AggregateType, &row.AggregateID,
		&row.AggregateVersion, &row.Disposition, &expected, &received, &eventBytes,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return inboxRow{}, false, nil
	}
	if err != nil {
		return inboxRow{}, false, fmt.Errorf("%w: lock inbox: %v", ErrTransient, err)
	}
	if expected.Valid {
		v := expected.Int64
		row.ExpectedVersion = &v
	}
	if received.Valid {
		v := received.Int64
		row.ReceivedVersion = &v
	}
	row.EventBytes = eventBytes
	return row, true, nil
}

func upsertInbox(ctx context.Context, tx *sql.Tx, row inboxRow) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO platform.inbox (
			consumer_name, event_id, tenant_id, content_hash,
			aggregate_type, aggregate_id, aggregate_version, disposition,
			expected_version, received_version, event_bytes, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, now()
		)
		ON CONFLICT (consumer_name, event_id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			content_hash = CASE
				WHEN EXCLUDED.disposition = 'quarantined_conflict' THEN platform.inbox.content_hash
				ELSE EXCLUDED.content_hash
			END,
			aggregate_type = EXCLUDED.aggregate_type,
			aggregate_id = EXCLUDED.aggregate_id,
			aggregate_version = EXCLUDED.aggregate_version,
			disposition = EXCLUDED.disposition,
			expected_version = EXCLUDED.expected_version,
			received_version = EXCLUDED.received_version,
			event_bytes = EXCLUDED.event_bytes,
			updated_at = now()
	`, ConsumerName, row.EventID, row.TenantID, row.ContentHash,
		row.AggregateType, row.AggregateID, row.AggregateVersion, row.Disposition,
		nullInt64(row.ExpectedVersion), nullInt64(row.ReceivedVersion), nullBytes(row.EventBytes),
	)
	if err != nil {
		return fmt.Errorf("%w: upsert inbox: %v", ErrTransient, err)
	}
	return nil
}

func quarantineInbox(ctx context.Context, db *sql.DB, env event.Envelope, raw []byte, disposition string, expected, received *int64) (Result, error) {
	hash, err := event.ContentHash(env)
	if err != nil {
		hash = ""
	}
	var canonical []byte
	if disposition == DispositionQuarantinedGap {
		canonical, _ = event.CanonicalBytes(env)
	}
	_ = raw

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("%w: begin: %v", ErrTransient, err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertInbox(ctx, tx, inboxRow{
		EventID:          env.EventID,
		TenantID:         env.TenantID,
		ContentHash:      hash,
		AggregateType:    env.AggregateType,
		AggregateID:      env.AggregateID,
		AggregateVersion: env.AggregateVersion,
		Disposition:      disposition,
		ExpectedVersion:  expected,
		ReceivedVersion:  received,
		EventBytes:       canonical,
	}); err != nil {
		return Result{}, err
	}
	if err := commitWithHook(ctx, tx); err != nil {
		return Result{}, err
	}
	return Result{
		Disposition: disposition,
		EventID:     env.EventID,
		ShouldAck:   true,
	}, nil
}

// testFailRedrive, when set by same-package tests, runs at the start of
// redriveGaps so crash/retry behavior can be exercised.
var testFailRedrive func() error

func setTestFailRedriveForTest(fn func() error) {
	testFailRedrive = fn
}

func redriveGaps(ctx context.Context, db *sql.DB, tenantID, aggregateType, aggregateID string) error {
	if testFailRedrive != nil {
		if err := testFailRedrive(); err != nil {
			return fmt.Errorf("%w: %v", ErrTransient, err)
		}
	}
	for {
		var (
			eventID string
			raw     []byte
			hash    string
		)
		err := db.QueryRowContext(ctx, `
			SELECT i.event_id, i.event_bytes, i.content_hash
			FROM platform.inbox i
			JOIN platform.inventory_projection p
			  ON p.tenant_id = i.tenant_id AND p.item_id = i.aggregate_id
			WHERE i.consumer_name = $1
			  AND i.tenant_id = $2
			  AND i.aggregate_type = $3
			  AND i.aggregate_id = $4
			  AND i.disposition = $5
			  AND i.aggregate_version = p.aggregate_version + 1
			  AND i.event_bytes IS NOT NULL
			ORDER BY i.aggregate_version
			LIMIT 1
		`, ConsumerName, tenantID, aggregateType, aggregateID, DispositionQuarantinedGap).Scan(&eventID, &raw, &hash)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: select gap: %v", ErrTransient, err)
		}

		env, err := event.Parse(raw)
		if err != nil {
			return fmt.Errorf("platform: redrive parse %s: %w", eventID, err)
		}
		gotHash, err := event.ContentHash(env)
		if err != nil {
			return err
		}
		if gotHash != hash {
			_, qerr := quarantineInbox(ctx, db, env, raw, DispositionQuarantinedConflict, nil, nil)
			if qerr != nil {
				return qerr
			}
			continue
		}

		res, err := processValidated(ctx, db, env, raw)
		if err != nil {
			return err
		}
		if res.Disposition != DispositionApplied {
			return nil
		}
	}
}

// persistParseFailure quarantines bytes that failed Parse. Recovered JSON
// identity is stored when present; payload bytes are not.
func persistParseFailure(ctx context.Context, db *sql.DB, value []byte, pos SourcePosition, parseErr error) (Result, error) {
	category := "malformed_envelope"
	code := "malformed_envelope"
	if errors.Is(parseErr, event.ErrUnsupported) {
		category = "unsupported_contract"
		code = "unsupported_contract"
	} else if errors.Is(parseErr, event.ErrMalformed) {
		category = "malformed_envelope"
		code = "malformed_envelope"
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("%w: begin: %v", ErrTransient, err)
	}
	defer func() { _ = tx.Rollback() }()

	eventID, tenantID, aggType, aggID, aggVer, schemaVer, eventType := recoveredParseIdentity(value)
	if err := upsertFailure(ctx, tx, failureRow{
		EventID:            eventID,
		TenantID:           tenantID,
		AggregateType:      aggType,
		AggregateID:        aggID,
		AggregateVersion:   aggVer,
		EventSchemaVersion: schemaVer,
		EventType:          eventType,
		FailureCategory:    category,
		DiagnosticCode:     code,
		ReceivedBytesHash:  receivedBytesHash(value),
		Source:             pos,
		AttemptCount:       1,
		QuarantineStatus:   "quarantined",
	}); err != nil {
		return Result{}, err
	}
	if err := commitWithHook(ctx, tx); err != nil {
		return Result{}, err
	}
	return Result{
		Disposition: DispositionQuarantinedInvalid,
		ShouldAck:   true,
	}, nil
}

// persistConsumerUnsupported quarantines an envelope that parsed as an accepted
// family this projection consumer does not apply. Identity fields Parse recovered
// are recorded so tenant-scoped inspect can see the quarantine.
func persistConsumerUnsupported(ctx context.Context, db *sql.DB, env event.Envelope, value []byte, pos SourcePosition) (Result, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("%w: begin: %v", ErrTransient, err)
	}
	defer func() { _ = tx.Rollback() }()

	eventID, tenantID, aggType, aggID, aggVer, schemaVer, eventType, hash := failureIdentity(&env)
	if err := upsertFailure(ctx, tx, failureRow{
		EventID:            eventID,
		TenantID:           tenantID,
		AggregateType:      aggType,
		AggregateID:        aggID,
		AggregateVersion:   aggVer,
		EventSchemaVersion: schemaVer,
		EventType:          eventType,
		FailureCategory:    "unsupported_contract",
		ContentHash:        hash,
		ReceivedBytesHash:  receivedBytesHash(value),
		Source:             pos,
		AttemptCount:       1,
		DiagnosticCode:     "unsupported_contract",
		QuarantineStatus:   "quarantined",
	}); err != nil {
		return Result{}, err
	}
	if err := commitWithHook(ctx, tx); err != nil {
		return Result{}, err
	}
	return Result{
		Disposition: DispositionQuarantinedInvalid,
		ShouldAck:   true,
	}, nil
}

// recoveredParseIdentity copies envelope identity fields from JSON that failed
// Parse. Payload is ignored. Unparseable input leaves every field nil.
func recoveredParseIdentity(value []byte) (eventID, tenantID, aggType, aggID *string, aggVer, schemaVer *int64, eventType *string) {
	var probe struct {
		EventID            string `json:"event_id"`
		TenantID           string `json:"tenant_id"`
		EventType          string `json:"event_type"`
		EventSchemaVersion *int64 `json:"event_schema_version"`
		AggregateType      string `json:"aggregate_type"`
		AggregateID        string `json:"aggregate_id"`
		AggregateVersion   *int64 `json:"aggregate_version"`
	}
	if json.Unmarshal(value, &probe) != nil {
		return
	}
	eventID = nonEmptyPtr(probe.EventID)
	tenantID = nonEmptyPtr(probe.TenantID)
	eventType = nonEmptyPtr(probe.EventType)
	aggType = nonEmptyPtr(probe.AggregateType)
	aggID = nonEmptyPtr(probe.AggregateID)
	if probe.AggregateVersion != nil && *probe.AggregateVersion > 0 {
		aggVer = probe.AggregateVersion
	}
	if probe.EventSchemaVersion != nil && *probe.EventSchemaVersion > 0 {
		schemaVer = probe.EventSchemaVersion
	}
	return
}

func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func failureIdentity(env *event.Envelope) (eventID, tenantID, aggType, aggID *string, aggVer, schemaVer *int64, eventType, hash *string) {
	if env == nil {
		return
	}
	eventID = &env.EventID
	tenantID = &env.TenantID
	aggType = &env.AggregateType
	aggID = &env.AggregateID
	aggVer = &env.AggregateVersion
	schemaVer = &env.EventSchemaVersion
	eventType = &env.EventType
	if h, herr := event.ContentHash(*env); herr == nil {
		hash = &h
	}
	return
}

// bumpPoisonAttempt records an explicit handler-poison attempt. Callers must
// not use it for retryable begin/commit/SQL failures from processValidated.
func bumpPoisonAttempt(ctx context.Context, db *sql.DB, env *event.Envelope, value []byte, pos SourcePosition, code string, cause error) (Result, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{ShouldAck: false}, fmt.Errorf("%w: begin poison: %v", ErrTransient, err)
	}
	defer func() { _ = tx.Rollback() }()

	eventID, tenantID, aggType, aggID, aggVer, schemaVer, eventType, hash := failureIdentity(env)

	attempts, status, err := upsertFailureReturning(ctx, tx, failureRow{
		EventID:            eventID,
		TenantID:           tenantID,
		AggregateType:      aggType,
		AggregateID:        aggID,
		AggregateVersion:   aggVer,
		EventSchemaVersion: schemaVer,
		EventType:          eventType,
		FailureCategory:    "handler_poison",
		ContentHash:        hash,
		ReceivedBytesHash:  receivedBytesHash(value),
		Source:             pos,
		DiagnosticCode:     code,
		QuarantineStatus:   "retrying",
	})
	if err != nil {
		return Result{ShouldAck: false}, err
	}

	if attempts >= MaxHandlerAttempts {
		status = "quarantined"
		if _, err := tx.ExecContext(ctx, `
			UPDATE platform.processing_failures
			SET quarantine_status = 'quarantined', updated_at = now()
			WHERE consumer_name = $1
			  AND source_topic = $2
			  AND source_partition = $3
			  AND source_offset = $4
		`, ConsumerName, pos.Topic, pos.Partition, pos.Offset); err != nil {
			return Result{ShouldAck: false}, fmt.Errorf("%w: quarantine poison: %v", ErrTransient, err)
		}
		if err := commitWithHook(ctx, tx); err != nil {
			return Result{ShouldAck: false}, err
		}
		return Result{
			Disposition: DispositionQuarantinedInvalid,
			ShouldAck:   true,
		}, fmt.Errorf("%w: %v", ErrPoison, cause)
	}

	if err := commitWithHook(ctx, tx); err != nil {
		return Result{ShouldAck: false}, err
	}
	_ = status
	return Result{ShouldAck: false}, fmt.Errorf("%w: %v", ErrTransient, cause)
}

type failureRow struct {
	EventID            *string
	TenantID           *string
	AggregateType      *string
	AggregateID        *string
	AggregateVersion   *int64
	EventSchemaVersion *int64
	EventType          *string
	FailureCategory    string
	ContentHash        *string
	ReceivedBytesHash  string
	Source             SourcePosition
	AttemptCount       int
	DiagnosticCode     string
	QuarantineStatus   string
}

func upsertFailure(ctx context.Context, tx *sql.Tx, row failureRow) error {
	_, _, err := upsertFailureReturning(ctx, tx, row)
	return err
}

func upsertFailureReturning(ctx context.Context, tx *sql.Tx, row failureRow) (attempts int, status string, err error) {
	failureID, err := newFailureID()
	if err != nil {
		return 0, "", err
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO platform.processing_failures (
			failure_id, consumer_name, event_id, tenant_id,
			aggregate_type, aggregate_id, aggregate_version,
			event_schema_version, event_type, failure_category,
			content_hash, received_bytes_hash,
			source_topic, source_partition, source_offset,
			attempt_count, diagnostic_code, quarantine_status
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			$11, $12,
			$13, $14, $15,
			1, $16, $17
		)
		ON CONFLICT (consumer_name, source_topic, source_partition, source_offset) DO UPDATE SET
			attempt_count = platform.processing_failures.attempt_count + 1,
			diagnostic_code = EXCLUDED.diagnostic_code,
			failure_category = EXCLUDED.failure_category,
			updated_at = now()
		RETURNING attempt_count, quarantine_status
	`, failureID, ConsumerName, nullString(row.EventID), nullString(row.TenantID),
		nullString(row.AggregateType), nullString(row.AggregateID), nullInt64(row.AggregateVersion),
		nullInt64(row.EventSchemaVersion), nullString(row.EventType), row.FailureCategory,
		nullString(row.ContentHash), nullStringPtr(row.ReceivedBytesHash),
		row.Source.Topic, row.Source.Partition, row.Source.Offset,
		row.DiagnosticCode, row.QuarantineStatus,
	).Scan(&attempts, &status)
	if err != nil {
		return 0, "", fmt.Errorf("%w: upsert failure: %v", ErrTransient, err)
	}
	return attempts, status, nil
}

func commitWithHook(ctx context.Context, tx *sql.Tx) error {
	if testFailBeforeCommit != nil {
		if err := testFailBeforeCommit(ctx); err != nil {
			return fmt.Errorf("%w: %v", ErrTransient, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit: %v", ErrTransient, err)
	}
	return nil
}

func isRetryableDB(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrTransient)
}

func receivedBytesHash(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func newFailureID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("%w: failure id: %v", ErrTransient, err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func ptrInt64(v int64) *int64 { return &v }

func nullInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func nullString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullStringPtr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ProjectionState returns the committed inventory projection row, if any.
func ProjectionState(ctx context.Context, db *sql.DB, tenantID, itemID string) (qty, version int64, ok bool, err error) {
	err = db.QueryRowContext(ctx, `
		SELECT quantity_on_hand, aggregate_version
		FROM platform.inventory_projection
		WHERE tenant_id = $1 AND item_id = $2
	`, tenantID, itemID).Scan(&qty, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return qty, version, true, nil
}

// InboxDisposition returns the inbox disposition for eventID, if present.
func InboxDisposition(ctx context.Context, db *sql.DB, eventID string) (disposition string, contentHash string, ok bool, err error) {
	err = db.QueryRowContext(ctx, `
		SELECT disposition, content_hash
		FROM platform.inbox
		WHERE consumer_name = $1 AND event_id = $2
	`, ConsumerName, eventID).Scan(&disposition, &contentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return disposition, contentHash, true, nil
}
