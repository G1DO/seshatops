/** Synthetic Northstar Foods fixture values for tests and demo defaults. */

export const NORTHSTAR_TENANT_ID = "11111111-1111-4111-8111-111111111111";
export const NORTHSTAR_ITEM_ID = "item-flour-001";
export const NORTHSTAR_EVENT_ID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1";
export const NORTHSTAR_BATCH_ID = "batch-bread-001";
export const NORTHSTAR_SUPPLIER_ID = "mill-northstar-001";
export const NORTHSTAR_LOT_ID = "lot-flour-2026-001";
export const NORTHSTAR_SHIPMENT_ID = "ship-northstar-001";
export const NORTHSTAR_LINEAGE_ORDER_ID = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3020";

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

export function sampleBatchTrace() {
  const hop = {
    id: NORTHSTAR_SUPPLIER_ID,
    parent_id: "",
    item_id: "",
    order_id: "",
    aggregate_version: 1,
    source_event_id: "218f5d78-6e64-4f5f-bd16-8e9f7c4a3011",
    event_schema_version: 1,
    occurred_at: "2026-08-07T10:00:00Z",
    recorded_at: "2026-08-07T10:00:00Z",
    correlation_id: "218f5d78-6e64-4f5f-bd16-8e9f7c4a3001",
    causation_id: null as string | null,
    trace_id: "218f5d78-6e64-4f5f-bd16-8e9f7c4a3002",
  };
  return {
    tenant_id: NORTHSTAR_TENANT_ID,
    observed_at: "2026-08-16T10:00:00.000Z",
    supplier: hop,
    lot: {
      ...hop,
      id: NORTHSTAR_LOT_ID,
      parent_id: NORTHSTAR_SUPPLIER_ID,
      item_id: NORTHSTAR_ITEM_ID,
      source_event_id: "218f5d78-6e64-4f5f-bd16-8e9f7c4a3012",
      occurred_at: "2026-08-07T10:01:00Z",
      recorded_at: "2026-08-07T10:01:00Z",
      causation_id: hop.source_event_id,
    },
    batch: {
      ...hop,
      id: NORTHSTAR_BATCH_ID,
      parent_id: NORTHSTAR_LOT_ID,
      source_event_id: "218f5d78-6e64-4f5f-bd16-8e9f7c4a3013",
      occurred_at: "2026-08-07T10:02:00Z",
      recorded_at: "2026-08-07T10:02:00Z",
      causation_id: "218f5d78-6e64-4f5f-bd16-8e9f7c4a3012",
    },
    shipment: {
      ...hop,
      id: NORTHSTAR_SHIPMENT_ID,
      parent_id: NORTHSTAR_BATCH_ID,
      order_id: NORTHSTAR_LINEAGE_ORDER_ID,
      source_event_id: "218f5d78-6e64-4f5f-bd16-8e9f7c4a3014",
      occurred_at: "2026-08-07T10:03:00Z",
      recorded_at: "2026-08-07T10:03:00Z",
      causation_id: "218f5d78-6e64-4f5f-bd16-8e9f7c4a3013",
    },
  };
}
