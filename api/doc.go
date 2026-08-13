// Package api exposes the Event Spine read-only Go REST and SSE surface for the
// committed inventory projection (Issue #27).
//
// The browser talks only to this Go API. PostgreSQL and Redpanda remain
// server-side. Issue #45 requires a fresh Go-owned session on /v1 routes;
// unauthenticated callers receive 401. Issue #46 requires MX-001 on inventory
// reads; unauthorized callers receive 403. SSE notifications are at-least-once
// and may be dropped for slow clients; REST is the authoritative
// reconnect/catch-up path.
package api
