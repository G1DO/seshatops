/** Synthetic Northstar Foods fixture values for tests and demo defaults. */

export const NORTHSTAR_TENANT_ID = "11111111-1111-4111-8111-111111111111";
export const NORTHSTAR_ITEM_ID = "item-flour-001";
export const NORTHSTAR_EVENT_ID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1";

/** Demo-only expected quantities from the deterministic M1 fixture narrative. */
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
