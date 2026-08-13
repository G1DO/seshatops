package platform

import "errors"

// Sentinel outcomes and validation failures for the Event Spine projection consumer.
var (
	// ErrTransient indicates a retryable processing failure. The caller must
	// not acknowledge the Redpanda offset.
	ErrTransient = errors.New("platform: transient processing failure")

	// ErrHandlerPoison marks a non-retryable handler fault that must consume the
	// poison attempt budget via bumpPoisonAttempt. It must not be used for
	// retryable PostgreSQL begin/commit/SQL failures.
	ErrHandlerPoison = errors.New("platform: handler poison")

	// ErrPoison indicates the delivery exceeded the Event Spine handler-attempt budget
	// and a sanitized failure record was persisted.
	ErrPoison = errors.New("platform: poison delivery quarantined")

	// ErrNotReleasable means an authorized operator asked to force-apply a
	// terminal inbox quarantine (gap, conflict, stale, invalid, mismatch,
	// transition) or a poison row with no retained same-tenant bytes.
	ErrNotReleasable = errors.New("platform: quarantine not releasable")

	// ErrControlNotFound means no same-tenant quarantined outbox or poison
	// row matched the requested event identity.
	ErrControlNotFound = errors.New("platform: control target not found")

	// ErrTenantMismatch means retained event bytes named a different tenant
	// than the authorized path tenant.
	ErrTenantMismatch = errors.New("platform: retained history tenant mismatch")
)
