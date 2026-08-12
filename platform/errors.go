package platform

import "errors"

// Sentinel outcomes and validation failures for the M1 projection consumer.
var (
	// ErrTransient indicates a retryable processing failure. The caller must
	// not acknowledge the Redpanda offset.
	ErrTransient = errors.New("platform: transient processing failure")

	// ErrPoison indicates the delivery exceeded the M1 handler-attempt budget
	// and a sanitized failure record was persisted.
	ErrPoison = errors.New("platform: poison delivery quarantined")
)
