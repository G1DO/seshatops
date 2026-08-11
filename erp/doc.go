// Package erp implements the M1 synthetic-ERP source transaction: one-line
// order acceptance, authoritative inventory update, and immutable outbox
// insert in a single PostgreSQL transaction (CONTRACTS.md §4).
//
// Broker publication is performed asynchronously by package relay (Issue #24).
// AcceptOrder does not call or require Redpanda; outbox rows remain pending
// until the relay publishes them.
package erp
