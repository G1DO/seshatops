/** DTOs aligned with Go api package JSON tags and openapi-projection.yaml. */

export interface InventoryItem {
  item_id: string;
  quantity_on_hand: number;
  aggregate_version: number;
}

export interface InventorySnapshot {
  tenant_id: string;
  items: InventoryItem[];
  checksum: string;
  observed_at: string;
}

export interface ProjectionUpdated {
  tenant_id: string;
  item_id: string;
  quantity_on_hand: number;
  aggregate_version: number;
  last_applied_event_id: string;
  checksum: string;
}

export interface ErrorBody {
  error: string;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string) {
    super(code);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export const UNAUTHENTICATED = "unauthenticated";

/** Presentation-only session fields from GET /auth/session. Not authorization. */
export interface SessionView {
  principal_id: string;
  subject: string;
  issuer: string;
  authenticated_at: string;
  expires_at: string;
  correlation_id: string;
}
