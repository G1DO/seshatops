import { describe, expect, it } from "vitest";
import {
  applySnapshot,
  applySseUpdate,
  emptyProjectionView,
} from "./projectionStore";
import {
  sampleSnapshotAfter,
  sampleSnapshotBefore,
  sampleSseUpdate,
} from "../fixtures/northstar";

describe("projectionStore", () => {
  it("applies REST snapshot without inventing a before value on first load", () => {
    const next = applySnapshot(emptyProjectionView(), sampleSnapshotBefore());
    expect(next.items).toHaveLength(1);
    expect(next.items[0]?.previous_quantity_on_hand).toBeNull();
    expect(next.items[0]?.quantity_on_hand).toBe(10);
    expect(next.checksum).toBe(sampleSnapshotBefore().checksum);
    expect(next.observed_at).toBe(sampleSnapshotBefore().observed_at);
  });

  it("records presentation before/after across SSE update", () => {
    const loaded = applySnapshot(emptyProjectionView(), sampleSnapshotBefore());
    const updated = applySseUpdate(loaded, sampleSseUpdate());
    expect(updated.items[0]?.previous_quantity_on_hand).toBe(10);
    expect(updated.items[0]?.quantity_on_hand).toBe(8);
    expect(updated.last_applied_event_id).toBe(
      sampleSseUpdate().last_applied_event_id,
    );
    expect(updated.checksum).toBe(sampleSseUpdate().checksum);
  });

  it("does not invent a second transition for duplicate SSE payloads", () => {
    const loaded = applySnapshot(emptyProjectionView(), sampleSnapshotBefore());
    const once = applySseUpdate(loaded, sampleSseUpdate());
    const twice = applySseUpdate(once, sampleSseUpdate());
    expect(twice.items[0]?.previous_quantity_on_hand).toBe(10);
    expect(twice.items[0]?.quantity_on_hand).toBe(8);
    expect(twice.items).toHaveLength(1);
  });

  it("replaces state from REST catch-up while preserving prior display qty", () => {
    const loaded = applySnapshot(emptyProjectionView(), sampleSnapshotBefore());
    const withEvent = applySseUpdate(loaded, sampleSseUpdate());
    expect(withEvent.last_applied_event_id).toBe(
      sampleSseUpdate().last_applied_event_id,
    );
    const caught = applySnapshot(withEvent, sampleSnapshotAfter());
    expect(caught.items[0]?.previous_quantity_on_hand).toBe(10);
    expect(caught.items[0]?.quantity_on_hand).toBe(8);
    expect(caught.observed_at).toBe(sampleSnapshotAfter().observed_at);
    expect(caught.last_applied_event_id).toBeNull();
  });
});
