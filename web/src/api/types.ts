/** DTOs aligned with Go api package JSON tags and docs/api/openapi-projection.yaml. */

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
export const FORBIDDEN = "forbidden";

/** Presentation-only session fields from GET /auth/session. Not authorization. */
export interface SessionView {
  principal_id: string;
  subject: string;
  issuer: string;
  authenticated_at: string;
  expires_at: string;
  correlation_id: string;
}

/** Authorized lag/poison/freshness snapshot from GET .../ops. */
export interface OpsQuarantineSample {
  event_id: string;
  last_error_code: string;
  created_at: string;
}

export interface OpsFailureSample {
  failure_id: string;
  event_id: string;
  failure_category: string;
  diagnostic_code: string;
  quarantine_status: string;
  source_topic: string;
  source_partition: number;
  source_offset: number;
  attempt_count: number;
  created_at: string;
}

export interface OpsGapSample {
  event_id: string;
  tenant_id: string;
  aggregate_type: string;
  aggregate_id: string;
  aggregate_version: number;
  expected_version: number;
  received_version: number;
  created_at: string;
}

export interface OpsSnapshot {
  tenant_id: string;
  observed_at: string;
  projection: {
    checksum: string;
    item_count: number;
  };
  backlog: {
    pending: number;
    publishing: number;
    published: number;
    quarantined: number;
    oldest_unpublished: string | null;
    quarantines: OpsQuarantineSample[];
  };
  processing: {
    applied: number;
    duplicate_noop: number;
    quarantined_conflict: number;
    quarantined_gap: number;
    quarantined_stale: number;
    quarantined_invalid: number;
    quarantined_mismatch: number;
    quarantined_transition: number;
    failures_retrying: number;
    failures_quarantined: number;
    oldest_gap: string | null;
    oldest_failure: string | null;
    failures: OpsFailureSample[];
    gaps: OpsGapSample[];
  };
}

/** Authorized outcome of a privileged quarantine/replay/rebuild POST. */
export interface ControlResult {
  tenant_id: string;
  control: string;
  event_id?: string;
  status: string;
  disposition?: string;
  applied: number;
  duplicate_noop: number;
  quarantined: number;
  checksum?: string;
	incomplete_reasons?: string[];
}

/** One hop in the authorized batch lineage chain. */
export interface LineageHop {
  id: string;
  parent_id: string;
  item_id: string;
  order_id: string;
  aggregate_version: number;
  source_event_id: string;
  event_schema_version: number;
  occurred_at: string;
  recorded_at: string;
  correlation_id: string;
  causation_id: string | null;
  trace_id: string;
}

/** Authorized GET .../ops/lineage/batches/{batch_id} snapshot. */
export interface BatchTraceSnapshot {
  tenant_id: string;
  observed_at: string;
  supplier: LineageHop;
  lot: LineageHop;
  batch: LineageHop;
  shipment: LineageHop;
}
