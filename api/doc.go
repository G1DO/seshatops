// Package api exposes the Event Spine Go REST and SSE surface for the
// committed inventory projection (Issue #27) plus Issue #48 privileged
// quarantine/replay/rebuild POSTs and Issue #49 append-only audit read.
//
// The browser talks only to this Go API. PostgreSQL and Redpanda remain
// server-side. Issue #45 requires a fresh Go-owned session on /v1 routes;
// unauthenticated callers receive 401. Issue #46 requires MX-001 on inventory
// reads; Issue #47 requires MX-002 or MX-003 on GET .../ops. Issue #48
// requires MX-004/MX-005/MX-006 on privileged POSTs. Issue #49 requires
// MX-007 on GET .../ops/audit and persists privileged allows/denies.
// Unauthorized callers receive 403. SSE notifications are at-least-once and
// may be dropped for slow clients; REST is the authoritative reconnect/catch-up path.
package api
