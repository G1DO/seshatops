package forecast_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/forecast"
)

func TestBuildFeatureSnapshotIsDeterministicAndFeatureOnly(t *testing.T) {
	history := officialHistory(t)
	boundary := forecast.SourceBoundary{
		HistoryChecksum:    "history-a",
		ProjectionChecksum: "projection-a",
		MaxRecordedAt:      "2026-04-26T23:59:59Z",
		AppliedEventCount:  336,
	}

	a, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, boundary)
	if err != nil {
		t.Fatal(err)
	}
	shuffled := forecast.History{Seed: history.Seed, Observations: append([]forecast.Observation(nil), history.Observations...)}
	for i, j := 0, len(shuffled.Observations)-1; i < j; i, j = i+1, j-1 {
		shuffled.Observations[i], shuffled.Observations[j] = shuffled.Observations[j], shuffled.Observations[i]
	}
	b, err := forecast.BuildFeatureSnapshot(shuffled, forecast.TenantNS001, boundary)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != forecast.SnapshotStatusComplete || len(a.Rows) == 0 {
		t.Fatalf("snapshot status=%s rows=%d", a.Status, len(a.Rows))
	}
	if a.Checksum != b.Checksum || a.SnapshotID != b.SnapshotID {
		t.Fatalf("shuffle changed identity: a=%s/%s b=%s/%s", a.Checksum, a.SnapshotID, b.Checksum, b.SnapshotID)
	}
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "label") {
		t.Fatalf("feature snapshot contains label: %s", raw)
	}
}

func TestFeatureSnapshotFutureMutationDoesNotChangeEarlierRow(t *testing.T) {
	history := officialHistory(t)
	base, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{HistoryChecksum: "h"})
	if err != nil {
		t.Fatal(err)
	}
	mutated := forecast.History{Seed: history.Seed, Observations: append([]forecast.Observation(nil), history.Observations...)}
	for i := range mutated.Observations {
		if mutated.Observations[i].TenantID == forecast.TenantNS001 &&
			mutated.Observations[i].ItemID == forecast.ItemFlour &&
			mutated.Observations[i].AsOfDate == "2026-01-13" {
			mutated.Observations[i].QuantityOnHand++
		}
	}
	changed, err := forecast.BuildFeatureSnapshot(mutated, forecast.TenantNS001, forecast.SourceBoundary{HistoryChecksum: "h"})
	if err != nil {
		t.Fatal(err)
	}
	baseRow := featureRow(t, base, forecast.ItemFlour, "2026-01-05")
	changedRow := featureRow(t, changed, forecast.ItemFlour, "2026-01-05")
	if *baseRow != *changedRow {
		t.Fatalf("future mutation changed earlier row: base=%+v changed=%+v", *baseRow, *changedRow)
	}
}

func TestFeatureSnapshotHonorsDeclaredSourceCutoff(t *testing.T) {
	history := officialHistory(t)
	boundary := forecast.SourceBoundary{HistoryChecksum: "h", SourceCutoffDate: "2026-03-01"}
	base, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, boundary)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range base.Rows {
		if row.AsOfDate > boundary.SourceCutoffDate || row.SourceCutoffDate != row.AsOfDate {
			t.Fatalf("row escaped cutoff: %+v", row)
		}
	}
	mutated := forecast.History{Seed: history.Seed, Observations: append([]forecast.Observation(nil), history.Observations...)}
	for i := range mutated.Observations {
		if mutated.Observations[i].AsOfDate > boundary.SourceCutoffDate {
			mutated.Observations[i].QuantityOnHand += int64(i + 1)
		}
	}
	changed, err := forecast.BuildFeatureSnapshot(mutated, forecast.TenantNS001, boundary)
	if err != nil {
		t.Fatal(err)
	}
	if base.Checksum != changed.Checksum || base.SnapshotID != changed.SnapshotID {
		t.Fatalf("post-cutoff mutation changed snapshot: %s/%s vs %s/%s", base.Checksum, base.SnapshotID, changed.Checksum, changed.SnapshotID)
	}
}

func TestBuildFeatureSnapshotShortHistoryIsInsufficient(t *testing.T) {
	history := forecast.History{Observations: dense(forecast.TenantNS001, forecast.ItemFlour, "2026-01-05", []int64{5, 4, 3, 2, 1, 0, 0})}
	snapshot, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != forecast.SnapshotStatusInsufficient || len(snapshot.Rows) != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestBuildFeatureSnapshotRejectsInvalidHistory(t *testing.T) {
	observations := dense(forecast.TenantNS001, forecast.ItemFlour, "2026-01-05", []int64{5, 4, 3, 2, 1, 0, 0, 0})
	observations[2].QuantityOnHand = -1
	_, err := forecast.BuildFeatureSnapshot(forecast.History{Observations: observations}, forecast.TenantNS001, forecast.SourceBoundary{})
	if !errors.Is(err, forecast.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildFeatureSnapshotRejectsMalformedObservations(t *testing.T) {
	base := dense(forecast.TenantNS001, forecast.ItemFlour, "2026-01-05", []int64{5, 4, 3, 2, 1, 0, 0, 0})
	tests := []struct {
		name   string
		mutate func([]forecast.Observation) []forecast.Observation
	}{
		{name: "empty observation tenant", mutate: func(observations []forecast.Observation) []forecast.Observation {
			observations[0].TenantID = " "
			return observations
		}},
		{name: "invalid date", mutate: func(observations []forecast.Observation) []forecast.Observation {
			observations[0].AsOfDate = "2026-01-05T00:00:00Z"
			return observations
		}},
		{name: "duplicate", mutate: func(observations []forecast.Observation) []forecast.Observation {
			return append(observations, observations[0])
		}},
		{name: "calendar gap", mutate: func(observations []forecast.Observation) []forecast.Observation {
			return append(observations[:3], observations[4:]...)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observations := append([]forecast.Observation(nil), base...)
			_, err := forecast.BuildFeatureSnapshot(forecast.History{Observations: tt.mutate(observations)}, forecast.TenantNS001, forecast.SourceBoundary{})
			if !errors.Is(err, forecast.ErrInvalidInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestBuildFeatureSnapshotRejectsMalformedTenant(t *testing.T) {
	history := forecast.History{Observations: dense(forecast.TenantNS001, forecast.ItemFlour, "2026-01-05", []int64{5, 4, 3, 2, 1, 0, 0, 0})}
	_, err := forecast.BuildFeatureSnapshot(history, "tenant-not-a-uuid", forecast.SourceBoundary{})
	if !errors.Is(err, forecast.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestFeatureSnapshotBoundaryChangesIdentity(t *testing.T) {
	history := officialHistory(t)
	a, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{HistoryChecksum: "a"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{HistoryChecksum: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if a.SnapshotID == b.SnapshotID || a.Checksum == b.Checksum {
		t.Fatal("source boundary did not change snapshot identity")
	}
}

func featureRow(t *testing.T, snapshot forecast.FeatureSnapshot, item, date string) *forecast.FeatureRow {
	t.Helper()
	for i := range snapshot.Rows {
		if snapshot.Rows[i].ItemID == item && snapshot.Rows[i].AsOfDate == date {
			return &snapshot.Rows[i]
		}
	}
	t.Fatalf("missing feature row %s %s", item, date)
	return nil
}
