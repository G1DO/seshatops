// Package event implements the Event Spine UTF-8 JSON event envelope from docs/design/specifications/event-spine.md:
// strict parse/validation, RFC 8785 JCS canonicalization, content hashing, and
// same-event_id content conflict detection for the accepted v1 families
// (inventory.quantity_decremented and the M3 traceability allow-list).
package event
