package event

import "errors"

// Sentinel validation and identity errors for the M1 contract.
var (
	// ErrMalformed is returned for incomplete envelopes, invalid types/values,
	// duplicate object members, or non-JCS-compatible JSON.
	ErrMalformed = errors.New("event: malformed envelope")

	// ErrUnsupported is returned for unknown event types or schema versions.
	ErrUnsupported = errors.New("event: unsupported event type or schema version")

	// ErrIdentityConflict is returned when the same event_id appears with
	// different canonical content.
	ErrIdentityConflict = errors.New("event: event identity content conflict")
)
