// Package platform implements the Event Spine Go consumer that validates Redpanda
// events, commits inbox/deduplication state with the inventory and lineage
// projections in one PostgreSQL transaction, and acknowledges broker offsets
// only after that durable decision commits (docs/design/specifications/event-spine.md §§4–8).
//
// Delivery is at least once. Identical redelivery is a durable no-op.
// Conflicting reuse of event_id is an integrity failure. Malformed,
// unsupported, and handler-poison deliveries persist sanitized
// processing_failures rows. InspectProcessing exposes bounded failure/gap
// visibility for Event Spine verification. InspectProcessingForTenant is the
// tenant-scoped helper used by GET .../ops. After an applied projection
// commit, SetAppliedNotifier may receive a non-blocking AppliedUpdate for the
// read API.
//
// ResetDerivedState and RebuildFromHistory clear only derived platform state
// and replay retained event bytes through the same projection handler,
// comparing event-spine.md §8 inventory and lineage checksums.
// ResetDerivedStateForTenant, ReplayTenantHistory, and RebuildTenantFromHistory
// are the tenant-scoped operator helpers. This package does not claim
// exactly-once delivery or processing, and does not own backup/restore.
package platform
