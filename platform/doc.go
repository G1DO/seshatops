// Package platform implements the M1 Go consumer that validates Redpanda
// events, commits inbox/deduplication state with the inventory projection in
// one PostgreSQL transaction, and acknowledges broker offsets only after that
// durable decision commits (CONTRACTS.md §§4–8).
//
// Delivery is at least once. Identical redelivery is a durable no-op.
// Conflicting reuse of event_id is an integrity failure. This package does not
// claim exactly-once delivery or processing.
package platform
