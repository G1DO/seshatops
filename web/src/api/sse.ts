import type { ProjectionUpdated } from "./types";

export const EVENT_PROJECTION_UPDATED = "inventory_projection.updated";

export interface SseHandlers {
  onOpen: () => void;
  onUpdate: (update: ProjectionUpdated) => void;
  onDisconnected: () => void;
}

function isProjectionUpdated(value: unknown): value is ProjectionUpdated {
  if (value === null || typeof value !== "object") {
    return false;
  }
  const body = value as Record<string, unknown>;
  return (
    typeof body.tenant_id === "string" &&
    typeof body.item_id === "string" &&
    typeof body.quantity_on_hand === "number" &&
    Number.isFinite(body.quantity_on_hand) &&
    typeof body.aggregate_version === "number" &&
    Number.isFinite(body.aggregate_version) &&
    typeof body.last_applied_event_id === "string" &&
    typeof body.checksum === "string"
  );
}

export type EventSourceFactory = (
  url: string,
  init?: EventSourceInit,
) => EventSource;

/** Subscribe to committed projection updates. Heartbeats/comments are ignored. */
export function openProjectionStream(
  url: string,
  handlers: SseHandlers,
  createEventSource: EventSourceFactory = (u) =>
    new EventSource(u, { withCredentials: true }),
): () => void {
  const source = createEventSource(url);
  let closed = false;

  const disconnect = () => {
    if (closed) {
      return;
    }
    closed = true;
    source.close();
    handlers.onDisconnected();
  };

  source.onopen = () => {
    if (closed) {
      return;
    }
    handlers.onOpen();
  };

  source.addEventListener(EVENT_PROJECTION_UPDATED, (event: Event) => {
    if (closed) {
      return;
    }
    const message = event as MessageEvent<string>;
    let parsed: unknown;
    try {
      parsed = JSON.parse(message.data);
    } catch {
      disconnect();
      return;
    }
    if (!isProjectionUpdated(parsed)) {
      disconnect();
      return;
    }
    handlers.onUpdate(parsed);
  });

  source.onerror = () => {
    disconnect();
  };

  return () => {
    if (closed) {
      return;
    }
    closed = true;
    source.close();
  };
}
