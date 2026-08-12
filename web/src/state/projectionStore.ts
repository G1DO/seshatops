import type { InventoryItem, InventorySnapshot, ProjectionUpdated } from "../api/types";

/** Display row: API fields plus presentation-only previous quantity. */
export interface DisplayItem {
  item_id: string;
  quantity_on_hand: number;
  aggregate_version: number;
  /** Last rendered quantity before the latest authoritative update, if any. */
  previous_quantity_on_hand: number | null;
}

export interface ProjectionViewState {
  items: DisplayItem[];
  checksum: string;
  observed_at: string;
  last_applied_event_id: string | null;
}

export function emptyProjectionView(): ProjectionViewState {
  return {
    items: [],
    checksum: "",
    observed_at: "",
    last_applied_event_id: null,
  };
}

/**
 * Replace local state from an authoritative REST snapshot.
 * Preserves previous_quantity when the same item_id already had a different qty.
 */
export function applySnapshot(
  previous: ProjectionViewState,
  snapshot: InventorySnapshot,
): ProjectionViewState {
  const priorById = new Map(
    previous.items.map((item) => [item.item_id, item] as const),
  );
  const items: DisplayItem[] = snapshot.items.map((row) =>
    toDisplayItem(row, priorById.get(row.item_id)),
  );
  return {
    items,
    checksum: snapshot.checksum,
    observed_at: snapshot.observed_at,
    // REST snapshots do not carry last_applied_event_id; clear any prior SSE value.
    last_applied_event_id: null,
  };
}

/**
 * Merge one SSE update into local presentation state.
 * Duplicate identical payloads do not invent a second before/after transition.
 */
export function applySseUpdate(
  previous: ProjectionViewState,
  update: ProjectionUpdated,
): ProjectionViewState {
  const existing = previous.items.find((item) => item.item_id === update.item_id);
  const nextRow: InventoryItem = {
    item_id: update.item_id,
    quantity_on_hand: update.quantity_on_hand,
    aggregate_version: update.aggregate_version,
  };

  if (
    existing &&
    existing.quantity_on_hand === update.quantity_on_hand &&
    existing.aggregate_version === update.aggregate_version
  ) {
    return {
      ...previous,
      checksum: update.checksum,
      last_applied_event_id: update.last_applied_event_id,
    };
  }

  const display = toDisplayItem(nextRow, existing);
  const others = previous.items.filter((item) => item.item_id !== update.item_id);
  const items = [...others, display].sort((a, b) =>
    a.item_id.localeCompare(b.item_id),
  );

  return {
    items,
    checksum: update.checksum,
    observed_at: previous.observed_at,
    last_applied_event_id: update.last_applied_event_id,
  };
}

function toDisplayItem(
  row: InventoryItem,
  prior: DisplayItem | undefined,
): DisplayItem {
  if (!prior) {
    return {
      ...row,
      previous_quantity_on_hand: null,
    };
  }
  if (prior.quantity_on_hand === row.quantity_on_hand) {
    return {
      ...row,
      previous_quantity_on_hand: prior.previous_quantity_on_hand,
    };
  }
  return {
    ...row,
    previous_quantity_on_hand: prior.quantity_on_hand,
  };
}
