import {
  ApiError,
  type ErrorBody,
  type InventoryItem,
  type InventorySnapshot,
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
