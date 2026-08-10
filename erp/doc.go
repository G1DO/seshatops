// Package erp implements the M1 synthetic-ERP source transaction: one-line
// order acceptance, authoritative inventory update, and immutable outbox
// insert in a single PostgreSQL transaction (CONTRACTS.md §4).
//
// Broker publication is out of scope; outbox rows remain pending until a later
// source-owned relay (Issue #24) publishes them.
package erp
