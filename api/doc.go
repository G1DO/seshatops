// Package api exposes the Event Spine Go REST and SSE surface for the
// committed inventory projection, tenant-scoped batch lineage reads,
// privileged quarantine/replay/rebuild POSTs, and append-only audit read.
//
// The browser talks only to this Go API. PostgreSQL and Redpanda remain
// server-side. A fresh Go-owned session is required on /v1 routes
// (401). MX-001 covers inventory reads; MX-002 or MX-003 cover GET .../ops
// and GET .../ops/lineage/batches/{batch_id}; MX-004/MX-005/MX-006 cover
// privileged POSTs; MX-007 covers GET .../ops/audit and privileged allow/deny
// rows persist before mutation. Unauthorized callers receive 403. SSE
// notifications are at-least-once and may be dropped for slow clients; REST is
// the authoritative reconnect/catch-up path.
// Routes: docs/api/openapi-projection.yaml.
package api
