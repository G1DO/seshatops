// Package platform implements the Event Spine Go consumer that validates Redpanda
// events, commits inbox/deduplication state with the inventory projection in
// one PostgreSQL transaction, and acknowledges broker offsets only after that
// durable decision commits (CONTRACTS.md §§4–8).
//
// Delivery is at least once. Identical redelivery is a durable no-op.
// Conflicting reuse of event_id is an integrity failure. Malformed,
// unsupported, and handler-poison deliveries persist sanitized
// processing_failures rows. InspectProcessing exposes bounded failure/gap
// visibility for Event Spine verification. InspectProcessingForTenant is the
// tenant-scoped helper used by the Issue #47 ops product surface. After an
// applied projection commit, SetAppliedNotifier may receive a non-blocking
// AppliedUpdate for the Issue #27 read API.
//
// Issue #29 adds ResetDerivedState and RebuildFromHistory so tests can clear
// only derived platform state and replay retained event bytes through the same
// projection handler, comparing CONTRACTS.md §8 checksums. Issue #48 adds
// tenant-scoped ResetDerivedStateForTenant, ReplayTenantHistory, and
// RebuildTenantFromHistory for authorized operator controls. This package does
// not claim exactly-once delivery or processing, and does not own backup/restore
// (Traceability & Recovery) or a durable audit product (Issue #49, identity).
package platform
