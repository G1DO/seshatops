import {
  ApiError,
  type ControlResult,
  type ErrorBody,
  type InventoryItem,
  type InventorySnapshot,
  type OpsSnapshot,
} from "./types";

function isInventoryItem(value: unknown): value is InventoryItem {
  if (value === null || typeof value !== "object") {
    return false;
  }
  const row = value as Record<string, unknown>;
  return (
    typeof row.item_id === "string" &&
    typeof row.quantity_on_hand === "number" &&
    Number.isFinite(row.quantity_on_hand) &&
    typeof row.aggregate_version === "number" &&
    Number.isFinite(row.aggregate_version)
  );
}

function isInventorySnapshot(value: unknown): value is InventorySnapshot {
  if (value === null || typeof value !== "object") {
    return false;
  }
  const body = value as Record<string, unknown>;
  if (
    typeof body.tenant_id !== "string" ||
    typeof body.checksum !== "string" ||
    typeof body.observed_at !== "string" ||
    !Array.isArray(body.items)
  ) {
    return false;
  }
  return body.items.every(isInventoryItem);
}

function errorCodeFromBody(body: unknown): string | undefined {
  if (body === null || typeof body !== "object") {
    return undefined;
  }
  const err = (body as ErrorBody).error;
  return typeof err === "string" && err.length > 0 ? err : undefined;
}

/** Authoritative REST snapshot. Does not interpret business rules. */
export async function fetchSnapshot(
  baseUrl: string,
  tenantId: string,
  fetchImpl: typeof fetch = fetch,
): Promise<InventorySnapshot> {
  const root = baseUrl.replace(/\/$/, "");
  const url = `${root}/v1/tenants/${encodeURIComponent(tenantId)}/inventory`;
  let response: Response;
  try {
    response = await fetchImpl(url, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
  } catch {
    throw new ApiError(0, "network_error");
  }

  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new ApiError(response.status, "malformed_response");
  }

  if (!response.ok) {
    throw new ApiError(
      response.status,
      errorCodeFromBody(body) ?? "request_failed",
    );
  }

  if (!isInventorySnapshot(body)) {
    throw new ApiError(response.status, "malformed_response");
  }

  return body;
}

export function streamUrl(baseUrl: string, tenantId: string): string {
  const root = baseUrl.replace(/\/$/, "");
  return `${root}/v1/tenants/${encodeURIComponent(tenantId)}/inventory/stream`;
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function isOpsSnapshot(value: unknown): value is OpsSnapshot {
  if (value === null || typeof value !== "object") {
    return false;
  }
  const body = value as Record<string, unknown>;
  if (
    typeof body.tenant_id !== "string" ||
    typeof body.observed_at !== "string" ||
    body.projection === null ||
    typeof body.projection !== "object" ||
    body.backlog === null ||
    typeof body.backlog !== "object" ||
    body.processing === null ||
    typeof body.processing !== "object"
  ) {
    return false;
  }
  const projection = body.projection as Record<string, unknown>;
  const backlog = body.backlog as Record<string, unknown>;
  const processing = body.processing as Record<string, unknown>;
  return (
    typeof projection.checksum === "string" &&
    isFiniteNumber(projection.item_count) &&
    isFiniteNumber(backlog.pending) &&
    isFiniteNumber(backlog.publishing) &&
    isFiniteNumber(backlog.published) &&
    isFiniteNumber(backlog.quarantined) &&
    (backlog.oldest_unpublished === null ||
      typeof backlog.oldest_unpublished === "string") &&
    Array.isArray(backlog.quarantines) &&
    isFiniteNumber(processing.applied) &&
    isFiniteNumber(processing.quarantined_gap) &&
    isFiniteNumber(processing.failures_retrying) &&
    isFiniteNumber(processing.failures_quarantined) &&
    (processing.oldest_gap === null ||
      typeof processing.oldest_gap === "string") &&
    (processing.oldest_failure === null ||
      typeof processing.oldest_failure === "string") &&
    Array.isArray(processing.failures) &&
    Array.isArray(processing.gaps)
  );
}

/** Authorized lag/poison/freshness snapshot. Does not authorize. */
export async function fetchOps(
  baseUrl: string,
  tenantId: string,
  fetchImpl: typeof fetch = fetch,
): Promise<OpsSnapshot> {
  const root = baseUrl.replace(/\/$/, "");
  const url = `${root}/v1/tenants/${encodeURIComponent(tenantId)}/ops`;
  let response: Response;
  try {
    response = await fetchImpl(url, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
  } catch {
    throw new ApiError(0, "network_error");
  }

  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new ApiError(response.status, "malformed_response");
  }

  if (!response.ok) {
    throw new ApiError(
      response.status,
      errorCodeFromBody(body) ?? "request_failed",
    );
  }

  if (!isOpsSnapshot(body)) {
    throw new ApiError(response.status, "malformed_response");
  }

  return body;
}

function isControlResult(value: unknown): value is ControlResult {
  if (value === null || typeof value !== "object") {
    return false;
  }
  const body = value as Record<string, unknown>;
  return (
    typeof body.tenant_id === "string" &&
    typeof body.control === "string" &&
    typeof body.status === "string" &&
    isFiniteNumber(body.applied) &&
    isFiniteNumber(body.duplicate_noop) &&
    isFiniteNumber(body.quarantined)
  );
}

async function postControl(
  url: string,
  body: Record<string, string>,
  fetchImpl: typeof fetch,
): Promise<ControlResult> {
  let response: Response;
  try {
    response = await fetchImpl(url, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "network_error");
  }

  let parsed: unknown;
  try {
    parsed = await response.json();
  } catch {
    throw new ApiError(response.status, "malformed_response");
  }

  if (!response.ok) {
    throw new ApiError(
      response.status,
      errorCodeFromBody(parsed) ?? "request_failed",
    );
  }
  if (!isControlResult(parsed)) {
    throw new ApiError(response.status, "malformed_response");
  }
  return parsed;
}

/** Privileged quarantine release. Go authorizes; this client does not. */
export async function postQuarantineRelease(
  baseUrl: string,
  tenantId: string,
  eventId: string,
  fetchImpl: typeof fetch = fetch,
): Promise<ControlResult> {
  const root = baseUrl.replace(/\/$/, "");
  return postControl(
    `${root}/v1/tenants/${encodeURIComponent(tenantId)}/ops/quarantine/release`,
    { event_id: eventId },
    fetchImpl,
  );
}

/** Privileged same-tenant replay. Go authorizes; this client does not. */
export async function postReplay(
  baseUrl: string,
  tenantId: string,
  eventId?: string,
  fetchImpl: typeof fetch = fetch,
): Promise<ControlResult> {
  const root = baseUrl.replace(/\/$/, "");
  const body: Record<string, string> = {};
  if (eventId) {
    body.event_id = eventId;
  }
  return postControl(
    `${root}/v1/tenants/${encodeURIComponent(tenantId)}/ops/replay`,
    body,
    fetchImpl,
  );
}

/** Privileged same-tenant rebuild. Go authorizes; this client does not. */
export async function postRebuild(
  baseUrl: string,
  tenantId: string,
  fetchImpl: typeof fetch = fetch,
): Promise<ControlResult> {
  const root = baseUrl.replace(/\/$/, "");
  return postControl(
    `${root}/v1/tenants/${encodeURIComponent(tenantId)}/ops/rebuild`,
    {},
    fetchImpl,
  );
}
