package platform

import (
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
)

func TestReplayForecastEventsIsDeterministicAndVersionSafe(t *testing.T) {
	fixture, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	second := fixture.Event
	second.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b1"
	second.AggregateVersion = 2
	second.RecordedAt = "2026-08-08T09:00:00Z"
	second.OccurredAt = second.RecordedAt
	second.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b2"
	second.TraceID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b3"
	second.CausationID = &fixture.Event.EventID
	second.Payload = event.QuantityDecremented{
		OrderID:             "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b4",
		ItemID:              fixture.ItemID,
		QuantityDecremented: 1,
		QuantityBefore:      8,
		QuantityAfter:       7,
	}
	if err := event.Validate(second); err != nil {
		t.Fatal(err)
	}

	firstRecord := forecastSourceEvent{env: fixture.Event, contentHash: mustEventHash(t, fixture.Event), recordedAt: mustTime(t, fixture.Event.RecordedAt)}
	secondRecord := forecastSourceEvent{env: second, contentHash: mustEventHash(t, second), recordedAt: mustTime(t, second.RecordedAt)}
	a, stateA, reasonsA := replayForecastEvents([]forecastSourceEvent{secondRecord, firstRecord})
	b, stateB, reasonsB := replayForecastEvents([]forecastSourceEvent{firstRecord, secondRecord})
	if len(reasonsA) != 0 || len(reasonsB) != 0 {
		t.Fatalf("valid replay reasons: a=%v b=%v", reasonsA, reasonsB)
	}
	if len(a.Observations) != 2 || len(b.Observations) != 2 || a.Observations[0] != b.Observations[0] || a.Observations[1] != b.Observations[1] {
		t.Fatalf("replay order changed history: a=%+v b=%+v", a.Observations, b.Observations)
	}
	if stateA[fixture.ItemID] != stateB[fixture.ItemID] || stateA[fixture.ItemID].version != 2 || stateA[fixture.ItemID].observation.QuantityOnHand != 7 {
		t.Fatalf("replay state=%+v/%+v", stateA, stateB)
	}

	gap := second
	gap.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c1"
	gap.AggregateVersion = 4
	gap.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c2"
	gap.TraceID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c3"
	gap.CausationID = &second.EventID
	gap.Payload = event.QuantityDecremented{
		OrderID:             "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c4",
		ItemID:              fixture.ItemID,
		QuantityDecremented: 1,
		QuantityBefore:      7,
		QuantityAfter:       6,
	}
	if err := event.Validate(gap); err != nil {
		t.Fatal(err)
	}
	_, _, reasons := replayForecastEvents([]forecastSourceEvent{
		firstRecord,
		secondRecord,
		{env: gap, contentHash: mustEventHash(t, gap), recordedAt: mustTime(t, gap.RecordedAt)},
	})
	foundGap := false
	for _, reason := range reasons {
		if strings.Contains(reason, "aggregate version gap") {
			foundGap = true
		}
	}
	if !foundGap {
		t.Fatalf("missing version-gap reason: %v", reasons)
	}
}

func mustEventHash(t *testing.T, env event.Envelope) string {
	t.Helper()
	hash, err := event.ContentHash(env)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
