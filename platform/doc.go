// Package platform implements the M1 Go consumer that validates Redpanda
// events, commits inbox/deduplication state with the inventory projection in
// one PostgreSQL transaction, and acknowledges broker offsets only after that
// durable decision commits (CONTRACTS.md §§4–8).
//
// Delivery is at least once. Identical redelivery is a durable no-op.
// Conflicting reuse of event_id is an integrity failure. Malformed,
// unsupported, and handler-poison deliveries persist sanitized
// processing_failures rows. InspectProcessing exposes bounded failure/gap
// visibility for M1 verification. This package does not claim exactly-once
// delivery or processing, and does not own operator recovery controls.
package platform
