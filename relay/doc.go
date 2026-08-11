// Package relay implements the M1 source-owned outbox publisher: claim durable
// erp.outbox rows, publish exact stored event bytes to Redpanda, and record
// publication status only after broker acknowledgement (CONTRACTS.md §§4–5).
//
// Delivery is at least once. Duplicate publication with the same event identity
// and content is expected after the acknowledgement-before-status crash window.
// This package does not claim exactly-once delivery.
package relay
