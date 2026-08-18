package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/forecast"
)

// ForecastSource is the read-only, tenant-scoped operational input for a
// forecast feature snapshot. It contains no database handle and is safe to
// pass to pure forecast code.
type ForecastSource struct {
	History       forecast.History
	Boundary      forecast.SourceBoundary
	Status        string
	StatusReasons []string
}

type forecastSourceEvent struct {
	env         event.Envelope
	contentHash string
	recordedAt  time.Time
}

type forecastProjectionRow struct {
	tenantID string
	itemID   string
	quantity int64
	version  int64
}

type forecastReplayState struct {
	observation forecast.Observation
	version     int64
}

// LoadTenantForecastSource reads retained same-tenant inventory history and
// the current projection checkpoint in one read-only repeatable-read
// transaction. It never calls the projection mutator or writes derived state.
func LoadTenantForecastSource(ctx context.Context, db *sql.DB, tenantID string) (ForecastSource, error) {
	return LoadTenantForecastSourceAtCutoff(ctx, db, tenantID, time.Now().UTC())
}

// LoadTenantForecastSourceAtCutoff is the deterministic form used by tests and
// callers that declare an availability boundary. Events recorded after cutoff
// are outside the source and are not replay inputs.
func LoadTenantForecastSourceAtCutoff(ctx context.Context, db *sql.DB, tenantID string, cutoff time.Time) (ForecastSource, error) {
	if db == nil {
		return ForecastSource{}, fmt.Errorf("platform: forecast source: nil db")
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if tenantID == "" {
		return ForecastSource{}, fmt.Errorf("platform: forecast source: tenant_id required")
	}
	if cutoff.IsZero() {
		return ForecastSource{}, fmt.Errorf("platform: forecast source: cutoff required")
	}
	cutoff = cutoff.UTC()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return ForecastSource{}, fmt.Errorf("platform: forecast source begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	projection, projectionChecksum, err := loadForecastProjection(ctx, tx, tenantID)
	if err != nil {
		return ForecastSource{}, err
	}

	events, reasons, err := loadForecastEvents(ctx, tx, tenantID, cutoff)
	if err != nil {
		return ForecastSource{}, err
	}

	if err := detectForecastInboxHistoryGaps(ctx, tx, tenantID, cutoff, events, &reasons); err != nil {
		return ForecastSource{}, err
	}

	history, replayState, replayReasons := replayForecastEvents(events)
	reasons = append(reasons, replayReasons...)
	reasons = uniqueSortedReasons(reasons)

	if err := compareForecastProjection(projection, replayState, &reasons); err != nil {
		return ForecastSource{}, err
	}

	boundary := forecast.SourceBoundary{
		HistoryChecksum:    checksumForecastEvents(tenantID, events),
		ProjectionChecksum: projectionChecksum,
		AppliedEventCount:  len(events),
	}
	for _, record := range events {
		if boundary.MaxRecordedAt == "" || record.recordedAt.After(parseRecordedAt(boundary.MaxRecordedAt)) {
			boundary.MaxRecordedAt = record.env.RecordedAt
		}
	}
	if boundary.MaxRecordedAt != "" {
		if recordedAt := parseRecordedAt(boundary.MaxRecordedAt); !recordedAt.IsZero() {
			boundary.SourceCutoffDate = recordedAt.UTC().Format("2006-01-02")
		}
	}

	status := forecast.SnapshotStatusComplete
	if len(reasons) != 0 {
		status = forecast.SnapshotStatusIncomplete
		if hasOnlyStaleReasons(reasons) {
			status = forecast.SnapshotStatusStale
		}
	}

	source := ForecastSource{
		History:       history,
		Boundary:      boundary,
		Status:        status,
		StatusReasons: reasons,
	}
	if err := tx.Commit(); err != nil {
		return ForecastSource{}, fmt.Errorf("platform: forecast source commit: %w", err)
	}
	return source, nil
}

func loadForecastProjection(ctx context.Context, tx *sql.Tx, tenantID string) (map[string]forecastProjectionRow, string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT tenant_id, item_id, quantity_on_hand, aggregate_version
		FROM platform.inventory_projection
		WHERE tenant_id = $1
		ORDER BY item_id
	`, tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("platform: forecast projection query: %w", err)
	}
	defer rows.Close()

	projection := make(map[string]forecastProjectionRow)
	var list []forecastProjectionRow
	for rows.Next() {
		var row forecastProjectionRow
		if err := rows.Scan(&row.tenantID, &row.itemID, &row.quantity, &row.version); err != nil {
			return nil, "", fmt.Errorf("platform: forecast projection scan: %w", err)
		}
		row.tenantID = strings.ToLower(row.tenantID)
		row.itemID = strings.ToLower(row.itemID)
		projection[row.itemID] = row
		list = append(list, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("platform: forecast projection rows: %w", err)
	}
	return projection, checksumForecastProjection(list), nil
}

func loadForecastEvents(ctx context.Context, tx *sql.Tx, tenantID string, cutoff time.Time) ([]forecastSourceEvent, []string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT o.event_id, o.tenant_id, o.aggregate_type, o.aggregate_id,
		       o.aggregate_version, o.content_hash, o.event_bytes,
		       o.status, o.recorded_at,
		       i.tenant_id, i.aggregate_type, i.aggregate_id,
		       i.aggregate_version, i.content_hash, i.disposition
		FROM erp.outbox o
		LEFT JOIN platform.inbox i
		  ON i.consumer_name = $2 AND i.tenant_id = $1 AND i.event_id = o.event_id
		WHERE o.tenant_id = $1
		  AND o.recorded_at <= $3
		  AND o.aggregate_type = $4
		ORDER BY o.aggregate_id, o.aggregate_version, o.recorded_at, o.event_id
	`, tenantID, ConsumerName, cutoff, event.AggregateTypeInventoryItem)
	if err != nil {
		return nil, nil, fmt.Errorf("platform: forecast event query: %w", err)
	}
	defer rows.Close()

	var events []forecastSourceEvent
	var reasons []string
	seenEventIDs := make(map[string]struct{})
	for rows.Next() {
		var (
			rowEventID       string
			rowTenantID      string
			rowAggregateType string
			rowAggregateID   string
			rowAggregateVer  int64
			rowContentHash   string
			raw              []byte
			outboxStatus     string
			outboxRecordedAt time.Time
			inboxTenantID    sql.NullString
			inboxAggType     sql.NullString
			inboxAggID       sql.NullString
			inboxAggVersion  sql.NullInt64
			inboxHash        sql.NullString
			disposition      sql.NullString
		)
		if err := rows.Scan(&rowEventID, &rowTenantID, &rowAggregateType, &rowAggregateID, &rowAggregateVer, &rowContentHash, &raw, &outboxStatus, &outboxRecordedAt, &inboxTenantID, &inboxAggType, &inboxAggID, &inboxAggVersion, &inboxHash, &disposition); err != nil {
			return nil, nil, fmt.Errorf("platform: forecast event scan: %w", err)
		}
		env, err := event.Parse(raw)
		if err != nil {
			reasons = append(reasons, "incomplete:malformed retained event "+rowEventID)
			continue
		}
		if env.EventType != event.EventTypeQuantityDecremented {
			continue
		}
		if env.EventID != rowEventID || env.TenantID != tenantID || strings.ToLower(rowTenantID) != tenantID || env.AggregateType != rowAggregateType || env.AggregateID != rowAggregateID || env.AggregateVersion != rowAggregateVer {
			reasons = append(reasons, "incomplete:event identity or tenant mismatch "+rowEventID)
			continue
		}
		contentHash, err := event.ContentHash(env)
		if err != nil {
			reasons = append(reasons, "incomplete:event hash unavailable "+rowEventID)
			continue
		}
		if strings.ToLower(contentHash) != strings.ToLower(rowContentHash) {
			reasons = append(reasons, "incomplete:outbox content hash mismatch "+rowEventID)
			continue
		}
		recordedAt, err := time.Parse(time.RFC3339Nano, env.RecordedAt)
		if err != nil || !recordedAt.Equal(outboxRecordedAt) {
			reasons = append(reasons, "incomplete:recorded_at mismatch "+rowEventID)
			continue
		}

		if !inboxHash.Valid || !disposition.Valid {
			if outboxStatus == "pending" || outboxStatus == "publishing" || outboxStatus == "published" {
				reasons = append(reasons, "stale:inventory event not applied "+rowEventID)
			} else {
				reasons = append(reasons, "incomplete:inventory event has no platform disposition "+rowEventID)
			}
			continue
		}
		if !inboxTenantID.Valid || strings.ToLower(inboxTenantID.String) != tenantID ||
			!inboxAggType.Valid || inboxAggType.String != rowAggregateType ||
			!inboxAggID.Valid || inboxAggID.String != rowAggregateID ||
			!inboxAggVersion.Valid || inboxAggVersion.Int64 != rowAggregateVer {
			reasons = append(reasons, "incomplete:inbox event identity mismatch "+rowEventID)
			continue
		}
		if strings.ToLower(inboxHash.String) != strings.ToLower(rowContentHash) {
			reasons = append(reasons, "incomplete:inbox content hash mismatch "+rowEventID)
			continue
		}
		switch disposition.String {
		case DispositionApplied, DispositionDuplicateNoop:
			if _, duplicate := seenEventIDs[env.EventID]; duplicate {
				reasons = append(reasons, "incomplete:duplicate retained event "+rowEventID)
				continue
			}
			seenEventIDs[env.EventID] = struct{}{}
			events = append(events, forecastSourceEvent{env: env, contentHash: contentHash, recordedAt: recordedAt})
		case DispositionQuarantinedGap, DispositionQuarantinedStale:
			reasons = append(reasons, "incomplete:inventory event disposition "+disposition.String+" "+rowEventID)
		default:
			reasons = append(reasons, "incomplete:inventory event disposition "+disposition.String+" "+rowEventID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("platform: forecast event rows: %w", err)
	}
	return events, reasons, nil
}

func detectForecastInboxHistoryGaps(ctx context.Context, tx *sql.Tx, tenantID string, cutoff time.Time, events []forecastSourceEvent, reasons *[]string) error {
	known := make(map[string]struct{}, len(events))
	for _, record := range events {
		known[record.env.EventID] = struct{}{}
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT i.event_id, i.disposition
		FROM platform.inbox i
		LEFT JOIN erp.outbox o ON o.event_id = i.event_id AND o.tenant_id = $2
		WHERE i.consumer_name = $1
		  AND i.tenant_id = $2
		  AND i.aggregate_type = $3
		  AND (o.event_id IS NULL OR o.recorded_at <= $4)
	`, ConsumerName, tenantID, event.AggregateTypeInventoryItem, cutoff)
	if err != nil {
		return fmt.Errorf("platform: forecast inbox query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var eventID, disposition string
		if err := rows.Scan(&eventID, &disposition); err != nil {
			return fmt.Errorf("platform: forecast inbox scan: %w", err)
		}
		if _, ok := known[eventID]; !ok {
			*reasons = append(*reasons, "incomplete:inventory inbox event missing retained outbox "+eventID)
		}
		if disposition == DispositionQuarantinedGap || disposition == DispositionQuarantinedStale {
			*reasons = append(*reasons, "incomplete:inventory inbox disposition "+disposition+" "+eventID)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("platform: forecast inbox rows: %w", err)
	}
	return nil
}

func replayForecastEvents(events []forecastSourceEvent) (forecast.History, map[string]forecastReplayState, []string) {
	byItem := make(map[string][]forecastSourceEvent)
	for _, record := range events {
		byItem[record.env.AggregateID] = append(byItem[record.env.AggregateID], record)
	}

	var reasons []string
	var observations []forecast.Observation
	replayState := make(map[string]forecastReplayState)
	items := make([]string, 0, len(byItem))
	for item := range byItem {
		items = append(items, item)
	}
	sort.Strings(items)
	for _, item := range items {
		series := byItem[item]
		sort.Slice(series, func(i, j int) bool {
			if series[i].env.AggregateVersion != series[j].env.AggregateVersion {
				return series[i].env.AggregateVersion < series[j].env.AggregateVersion
			}
			return series[i].env.EventID < series[j].env.EventID
		})
		var current int64
		var expected int64 = 1
		byDate := make(map[string]forecast.Observation)
		var lastValid forecastReplayState
		haveLastValid := false
		for _, record := range series {
			payload, ok := event.AsQuantityDecremented(record.env)
			if !ok {
				reasons = append(reasons, "incomplete:inventory payload mismatch "+record.env.EventID)
				continue
			}
			if record.env.AggregateVersion != expected {
				reasons = append(reasons, fmt.Sprintf("incomplete:aggregate version gap %s expected %d received %d", item, expected, record.env.AggregateVersion))
				continue
			}
			if record.env.AggregateVersion == 1 {
				current = payload.QuantityBefore
			} else if payload.QuantityBefore != current {
				reasons = append(reasons, "incomplete:quantity transition mismatch "+record.env.EventID)
				continue
			}
			if payload.QuantityBefore-payload.QuantityDecremented != payload.QuantityAfter || payload.QuantityAfter < 0 {
				reasons = append(reasons, "incomplete:quantity arithmetic mismatch "+record.env.EventID)
				continue
			}
			current = payload.QuantityAfter
			day := record.recordedAt.UTC().Format("2006-01-02")
			observation := forecast.Observation{TenantID: record.env.TenantID, ItemID: item, AsOfDate: day, QuantityOnHand: current}
			byDate[day] = observation
			lastValid = forecastReplayState{observation: observation, version: record.env.AggregateVersion}
			haveLastValid = true
			expected++
		}
		for _, observation := range byDate {
			observations = append(observations, observation)
		}
		if haveLastValid {
			replayState[item] = lastValid
		}
	}
	sort.Slice(observations, func(i, j int) bool {
		if observations[i].TenantID != observations[j].TenantID {
			return observations[i].TenantID < observations[j].TenantID
		}
		if observations[i].ItemID != observations[j].ItemID {
			return observations[i].ItemID < observations[j].ItemID
		}
		return observations[i].AsOfDate < observations[j].AsOfDate
	})
	return forecast.History{Observations: observations}, replayState, reasons
}

func compareForecastProjection(projection map[string]forecastProjectionRow, replayState map[string]forecastReplayState, reasons *[]string) error {
	for item, row := range projection {
		replay, ok := replayState[item]
		if !ok {
			*reasons = append(*reasons, "incomplete:projection item missing retained history "+item)
			continue
		}
		if replay.observation.QuantityOnHand != row.quantity {
			*reasons = append(*reasons, "incomplete:projection quantity differs from replay "+item)
		}
		if replay.version != row.version {
			*reasons = append(*reasons, "incomplete:projection version differs from replay "+item)
		}
	}
	for item := range replayState {
		if _, ok := projection[item]; !ok {
			*reasons = append(*reasons, "incomplete:replayed item missing projection "+item)
		}
	}
	return nil
}

func checksumForecastEvents(tenantID string, events []forecastSourceEvent) string {
	var b strings.Builder
	ordered := append([]forecastSourceEvent(nil), events...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].env.AggregateID != ordered[j].env.AggregateID {
			return ordered[i].env.AggregateID < ordered[j].env.AggregateID
		}
		if ordered[i].env.AggregateVersion != ordered[j].env.AggregateVersion {
			return ordered[i].env.AggregateVersion < ordered[j].env.AggregateVersion
		}
		return ordered[i].env.EventID < ordered[j].env.EventID
	})
	for _, record := range ordered {
		b.WriteString(tenantID)
		b.WriteByte('\t')
		b.WriteString(record.env.AggregateID)
		b.WriteByte('\t')
		b.WriteString(formatInt(record.env.AggregateVersion))
		b.WriteByte('\t')
		b.WriteString(record.env.EventID)
		b.WriteByte('\t')
		b.WriteString(record.contentHash)
		b.WriteByte('\t')
		b.WriteString(record.env.RecordedAt)
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func checksumForecastProjection(rows []forecastProjectionRow) string {
	ordered := append([]forecastProjectionRow(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].tenantID != ordered[j].tenantID {
			return ordered[i].tenantID < ordered[j].tenantID
		}
		return ordered[i].itemID < ordered[j].itemID
	})
	var b strings.Builder
	for _, row := range ordered {
		b.WriteString(strings.ToLower(row.tenantID))
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(row.itemID))
		b.WriteByte('\t')
		b.WriteString(formatInt(row.quantity))
		b.WriteByte('\t')
		b.WriteString(formatInt(row.version))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func parseRecordedAt(value string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}

func uniqueSortedReasons(reasons []string) []string {
	seen := make(map[string]struct{}, len(reasons))
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if strings.TrimSpace(reason) == "" {
			continue
		}
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		out = append(out, reason)
	}
	sort.Strings(out)
	return out
}

func hasOnlyStaleReasons(reasons []string) bool {
	if len(reasons) == 0 {
		return false
	}
	for _, reason := range reasons {
		if !strings.HasPrefix(reason, "stale:") {
			return false
		}
	}
	return true
}
