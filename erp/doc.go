// Package erp implements the Event Spine synthetic-ERP source transaction:
// M3 lineage hops, one-line order acceptance, authoritative inventory update,
// and immutable outbox insert in a single PostgreSQL transaction
// (docs/design/specifications/event-spine.md §4).
//
// Broker publication is performed asynchronously by package relay.
// Source accepts do not call or require Redpanda; outbox rows remain pending
// until the relay publishes them.
package erp
