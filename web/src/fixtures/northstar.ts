/** Synthetic Northstar Foods fixture values for tests and demo defaults. */

export const NORTHSTAR_TENANT_ID = "11111111-1111-4111-8111-111111111111";
export const NORTHSTAR_ITEM_ID = "item-flour-001";
export const NORTHSTAR_EVENT_ID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1";

/** Demo-only expected quantities from the deterministic Event Spine fixture narrative. */
export const NORTHSTAR_QUANTITY_BEFORE = 10;
export const NORTHSTAR_QUANTITY_AFTER = 8;

export const SAMPLE_CHECKSUM_BEFORE =
  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
export const SAMPLE_CHECKSUM_AFTER =
  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";

export function sampleSnapshotBefore() {
  return {
    tenant_id: NORTHSTAR_TENANT_ID,
    items: [
      {
        item_id: NORTHSTAR_ITEM_ID,
        quantity_on_hand: NORTHSTAR_QUANTITY_BEFORE,
        aggregate_version: 0,
      },
    ],
    checksum: SAMPLE_CHECKSUM_BEFORE,
    observed_at: "2026-08-12T07:00:00Z",
  };
}

export function sampleSnapshotAfter() {
  return {
    tenant_id: NORTHSTAR_TENANT_ID,
    items: [
      {
        item_id: NORTHSTAR_ITEM_ID,
        quantity_on_hand: NORTHSTAR_QUANTITY_AFTER,
        aggregate_version: 1,
      },
    ],
    checksum: SAMPLE_CHECKSUM_AFTER,
    observed_at: "2026-08-12T07:01:00Z",
  };
}

export function sampleSseUpdate() {
  return {
    tenant_id: NORTHSTAR_TENANT_ID,
    item_id: NORTHSTAR_ITEM_ID,
    quantity_on_hand: NORTHSTAR_QUANTITY_AFTER,
    aggregate_version: 1,
    last_applied_event_id: NORTHSTAR_EVENT_ID,
    checksum: SAMPLE_CHECKSUM_AFTER,
  };
}

export function sampleOpsSnapshot() {
  return {
    tenant_id: NORTHSTAR_TENANT_ID,
    observed_at: "2026-08-13T06:00:00.000Z",
    projection: {
      checksum: SAMPLE_CHECKSUM_AFTER,
      item_count: 1,
    },
    backlog: {
      pending: 1,
      publishing: 0,
      published: 0,
      quarantined: 0,
      oldest_unpublished: "2026-08-13T05:59:00.000Z",
      quarantines: [],
    },
    processing: {
      applied: 1,
      duplicate_noop: 0,
      quarantined_conflict: 0,
      quarantined_gap: 1,
      quarantined_stale: 0,
      quarantined_invalid: 0,
      quarantined_mismatch: 0,
      quarantined_transition: 0,
      failures_retrying: 0,
      failures_quarantined: 0,
      oldest_gap: "2026-08-13T05:58:00.000Z",
      oldest_failure: null,
      failures: [],
      gaps: [
        {
          event_id: NORTHSTAR_EVENT_ID,
          tenant_id: NORTHSTAR_TENANT_ID,
          aggregate_type: "inventory_item",
          aggregate_id: NORTHSTAR_ITEM_ID,
          aggregate_version: 2,
          expected_version: 1,
          received_version: 2,
          created_at: "2026-08-13T05:58:00.000Z",
        },
      ],
    },
  };
}
