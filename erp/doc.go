// Package erp implements the Event Spine synthetic-ERP source transaction: one-line
// order acceptance, authoritative inventory update, and immutable outbox
// insert in a single PostgreSQL transaction (docs/CONTRACTS.md §4).
//
// Broker publication is performed asynchronously by package relay.
// AcceptOrder does not call or require Redpanda; outbox rows remain pending
// until the relay publishes them.
package erp
