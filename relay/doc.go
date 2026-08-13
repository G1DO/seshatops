// Package relay implements the Event Spine source-owned outbox publisher: claim durable
// erp.outbox rows, publish exact stored event bytes to Redpanda, and record
// publication status only after broker acknowledgement (CONTRACTS.md §§4–5).
//
// Delivery is at least once. Duplicate publication with the same event identity
// and content is expected after the acknowledgement-before-status crash window.
// This package does not claim exactly-once delivery.
// InspectBacklog is the Event Spine library verification surface.
// InspectBacklogForTenant is the tenant-scoped helper used by Issue #47.
// ReleaseQuarantined is the tenant-scoped outbox retry used by Issue #48.
package relay
