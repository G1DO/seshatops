// Package api exposes the Event Spine read-only Go REST and SSE surface for the
// committed inventory projection (Issue #27).
//
// The browser talks only to this Go API. PostgreSQL and Redpanda remain
// server-side. Authentication and authorization remain Identity & Operations scope. SSE
// notifications are at-least-once and may be dropped for slow clients; REST
// is the authoritative reconnect/catch-up path.
package api
