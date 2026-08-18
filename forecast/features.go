package forecast

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

var featureTenantUUIDv4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

const (
	// FeatureDefinitionVersion identifies the raw, cutoff-safe feature contract
	// exposed to read-only forecasting consumers.
	FeatureDefinitionVersion = "m4-raw-onhand-v1"

	// SnapshotContractVersion identifies the serialized feature snapshot shape.
	SnapshotContractVersion = 1

	SnapshotStatusComplete     = "complete"
	SnapshotStatusStale        = "stale"
	SnapshotStatusIncomplete   = "incomplete"
	SnapshotStatusInsufficient = "insufficient"
)

// SourceBoundary identifies the immutable source boundary used to build a
// feature snapshot. It is intentionally metadata only; feature values remain
// in the snapshot rows.
type SourceBoundary struct {
	HistoryChecksum    string `json:"history_checksum"`
	ProjectionChecksum string `json:"projection_checksum"`
	MaxRecordedAt      string `json:"max_recorded_at,omitempty"`
	SourceCutoffDate   string `json:"source_cutoff_date,omitempty"`
	AppliedEventCount  int    `json:"applied_event_count"`
}

// FeatureRow is one raw, cutoff-safe feature row. It contains no future label
// or model output. QuantityOnHand is the only v1 feature value.
type FeatureRow struct {
	RowID            string `json:"row_id"`
	TenantID         string `json:"tenant_id"`
	ItemID           string `json:"item_id"`
	AsOfDate         string `json:"as_of_date"`
	SourceCutoffDate string `json:"source_cutoff_date"`
	Split            string `json:"split"`
	QuantityOnHand   int64  `json:"quantity_on_hand"`
	HistoryHash      string `json:"history_hash"`
}

// FeatureSnapshot is an on-demand deterministic read result. Rows are empty
// unless Status is complete so callers cannot accidentally use partial data.
type FeatureSnapshot struct {
	ContractVersion          int            `json:"contract_version"`
	Status                   string         `json:"status"`
	TenantID                 string         `json:"tenant_id"`
	DatasetVersion           string         `json:"dataset_version"`
	FeatureDefinitionVersion string         `json:"feature_definition_version"`
	SourceBoundary           SourceBoundary `json:"source_history_boundary"`
	SnapshotID               string         `json:"snapshot_id"`
	Checksum                 string         `json:"checksum"`
	Rows                     []FeatureRow   `json:"rows"`
	StatusReasons            []string       `json:"status_reasons"`
}

// BuildFeatureSnapshot builds the v1 raw feature rows from declared history.
// BuildDataset validates the temporal protocol and establishes the same
// chronological observation rows used by the frozen M4 evaluation contract.
// Future values are used only to establish whether a protocol row is
// labelable; no future value is copied into a feature row.
func BuildFeatureSnapshot(history History, tenantID string, boundary SourceBoundary) (FeatureSnapshot, error) {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if tenantID == "" {
		return FeatureSnapshot{}, wrapInvalid("empty tenant")
	}
	if !featureTenantUUIDv4.MatchString(tenantID) {
		return FeatureSnapshot{}, wrapInvalid("malformed tenant %s", tenantID)
	}

	if err := validateFeatureObservations(history.Observations); err != nil {
		return FeatureSnapshot{}, err
	}
	boundary = normalizeBoundary(boundary)
	declared := history
	if boundary.SourceCutoffDate != "" {
		if _, err := parseDate(boundary.SourceCutoffDate); err != nil {
			return FeatureSnapshot{}, err
		}
		declared.Observations = make([]Observation, 0, len(history.Observations))
		for _, observation := range history.Observations {
			if observation.AsOfDate <= boundary.SourceCutoffDate {
				declared.Observations = append(declared.Observations, observation)
			}
		}
	}

	series, err := indexObservations(declared.Observations, tenantID)
	if err != nil {
		return FeatureSnapshot{}, err
	}
	if len(series) == 0 {
		return FeatureSnapshot{}, wrapInvalid("no observations for tenant %s", tenantID)
	}
	labelableDates, err := uniqueLabelableDates(series)
	if err != nil {
		return FeatureSnapshot{}, err
	}
	splits := assignSplits(labelableDates)
	keys := make([]seriesKey, 0, len(series))
	for key := range series {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].tenant != keys[j].tenant {
			return keys[i].tenant < keys[j].tenant
		}
		return keys[i].item < keys[j].item
	})

	snapshot := FeatureSnapshot{
		ContractVersion:          SnapshotContractVersion,
		Status:                   SnapshotStatusComplete,
		TenantID:                 tenantID,
		DatasetVersion:           ProtocolID,
		FeatureDefinitionVersion: FeatureDefinitionVersion,
		SourceBoundary:           boundary,
		Rows:                     make([]FeatureRow, 0, len(labelableDates)),
	}
	for _, key := range keys {
		days := series[key]
		for _, asOf := range sortedDates(days) {
			split, ok := splits[asOf]
			if !ok {
				continue
			}
			end, ok := addDays(asOf, HorizonDays)
			if !ok {
				return FeatureSnapshot{}, wrapInvalid("as_of_date %s", asOf)
			}
			if _, ok := days[end]; !ok {
				continue
			}
			snapshot.Rows = append(snapshot.Rows, FeatureRow{
				RowID:            RowID(key.tenant, key.item, asOf),
				TenantID:         key.tenant,
				ItemID:           key.item,
				AsOfDate:         asOf,
				SourceCutoffDate: asOf,
				Split:            split,
				QuantityOnHand:   days[asOf],
				HistoryHash:      historyHash(days, asOf),
			})
		}
	}

	if len(snapshot.Rows) == 0 {
		snapshot.Status = SnapshotStatusInsufficient
		snapshot.StatusReasons = []string{"no labelable protocol observations"}
	}
	return finalizeFeatureSnapshot(snapshot), nil
}

// NewNonCompleteFeatureSnapshot creates a metadata-only snapshot for a valid
// source read that cannot safely produce usable rows. It is used by the
// PostgreSQL adapter to preserve explicit stale/incomplete/insufficient state.
func NewNonCompleteFeatureSnapshot(status, tenantID string, boundary SourceBoundary, reasons ...string) FeatureSnapshot {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	status = strings.ToLower(strings.TrimSpace(status))
	if status != SnapshotStatusStale && status != SnapshotStatusIncomplete && status != SnapshotStatusInsufficient {
		status = SnapshotStatusIncomplete
	}
	snapshot := FeatureSnapshot{
		ContractVersion:          SnapshotContractVersion,
		Status:                   status,
		TenantID:                 tenantID,
		DatasetVersion:           ProtocolID,
		FeatureDefinitionVersion: FeatureDefinitionVersion,
		SourceBoundary:           normalizeBoundary(boundary),
		Rows:                     []FeatureRow{},
		StatusReasons:            cleanReasons(reasons),
	}
	return finalizeFeatureSnapshot(snapshot)
}

// FeatureChecksum returns the canonical checksum for a snapshot. SnapshotID
// is deliberately excluded to avoid a circular identity calculation.
func FeatureChecksum(snapshot FeatureSnapshot) string {
	var b strings.Builder
	b.WriteString(formatInt(int64(snapshot.ContractVersion)))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.Status))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.TenantID))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.DatasetVersion))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.FeatureDefinitionVersion))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.SourceBoundary.HistoryChecksum))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.SourceBoundary.ProjectionChecksum))
	b.WriteByte('\t')
	b.WriteString(snapshot.SourceBoundary.MaxRecordedAt)
	b.WriteByte('\t')
	b.WriteString(snapshot.SourceBoundary.SourceCutoffDate)
	b.WriteByte('\t')
	b.WriteString(formatInt(int64(snapshot.SourceBoundary.AppliedEventCount)))
	b.WriteByte('\n')

	rows := append([]FeatureRow(nil), snapshot.Rows...)
	sort.Slice(rows, func(i, j int) bool { return featureRowLess(rows[i], rows[j]) })
	for _, row := range rows {
		b.WriteString(strings.ToLower(row.RowID))
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(row.TenantID))
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(row.ItemID))
		b.WriteByte('\t')
		b.WriteString(row.AsOfDate)
		b.WriteByte('\t')
		b.WriteString(row.SourceCutoffDate)
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(row.Split))
		b.WriteByte('\t')
		b.WriteString(formatInt(row.QuantityOnHand))
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(row.HistoryHash))
		b.WriteByte('\n')
	}
	for _, reason := range cleanReasons(snapshot.StatusReasons) {
		b.WriteString("reason\t")
		b.WriteString(reason)
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func finalizeFeatureSnapshot(snapshot FeatureSnapshot) FeatureSnapshot {
	snapshot.SourceBoundary = normalizeBoundary(snapshot.SourceBoundary)
	snapshot.StatusReasons = cleanReasons(snapshot.StatusReasons)
	if snapshot.Rows == nil {
		snapshot.Rows = []FeatureRow{}
	}
	snapshot.Checksum = FeatureChecksum(snapshot)
	snapshot.SnapshotID = snapshotIdentity(snapshot)
	return snapshot
}

func snapshotIdentity(snapshot FeatureSnapshot) string {
	var b strings.Builder
	b.WriteString(formatInt(int64(snapshot.ContractVersion)))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.TenantID))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.DatasetVersion))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.FeatureDefinitionVersion))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.SourceBoundary.HistoryChecksum))
	b.WriteByte('\t')
	b.WriteString(strings.ToLower(snapshot.SourceBoundary.ProjectionChecksum))
	b.WriteByte('\t')
	b.WriteString(snapshot.SourceBoundary.MaxRecordedAt)
	b.WriteByte('\t')
	b.WriteString(snapshot.SourceBoundary.SourceCutoffDate)
	b.WriteByte('\t')
	b.WriteString(formatInt(int64(snapshot.SourceBoundary.AppliedEventCount)))
	b.WriteByte('\t')
	b.WriteString(snapshot.Checksum)
	b.WriteByte('\n')
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func normalizeBoundary(boundary SourceBoundary) SourceBoundary {
	boundary.HistoryChecksum = strings.ToLower(strings.TrimSpace(boundary.HistoryChecksum))
	boundary.ProjectionChecksum = strings.ToLower(strings.TrimSpace(boundary.ProjectionChecksum))
	boundary.MaxRecordedAt = strings.TrimSpace(boundary.MaxRecordedAt)
	boundary.SourceCutoffDate = strings.TrimSpace(boundary.SourceCutoffDate)
	if boundary.AppliedEventCount < 0 {
		boundary.AppliedEventCount = 0
	}
	return boundary
}

func validateFeatureObservations(observations []Observation) error {
	seen := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		tenant := strings.ToLower(strings.TrimSpace(observation.TenantID))
		if tenant == "" {
			return wrapInvalid("empty observation tenant")
		}
		if !featureTenantUUIDv4.MatchString(tenant) {
			return wrapInvalid("malformed observation tenant %s", tenant)
		}
		item := strings.ToLower(strings.TrimSpace(observation.ItemID))
		if item == "" {
			return wrapInvalid("empty item_id")
		}
		if observation.QuantityOnHand < 0 {
			return wrapInvalid("negative quantity for %s %s", item, observation.AsOfDate)
		}
		day, err := parseDate(observation.AsOfDate)
		if err != nil {
			return err
		}
		date := day.Format(dateLayout)
		key := tenant + "\t" + item + "\t" + date
		if _, exists := seen[key]; exists {
			return wrapInvalid("duplicate observation %s %s %s", tenant, item, date)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func cleanReasons(reasons []string) []string {
	seen := make(map[string]struct{}, len(reasons))
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
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

func featureRowLess(a, b FeatureRow) bool {
	if a.TenantID != b.TenantID {
		return a.TenantID < b.TenantID
	}
	if a.ItemID != b.ItemID {
		return a.ItemID < b.ItemID
	}
	return a.AsOfDate < b.AsOfDate
}
