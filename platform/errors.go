package platform

import "errors"

// Sentinel outcomes and validation failures for the Event Spine projection consumer.
var (
	// ErrPythonUnavailable means the learned forecaster could not be invoked.
	ErrPythonUnavailable = errors.New("platform: python unavailable")

	// ErrPythonTimeout means the learned forecaster exceeded its deadline.
	ErrPythonTimeout = errors.New("platform: python timeout")

	// ErrPythonInvalidResponse means the learned forecaster returned a response
	// that failed the typed contract or output limit.
	ErrPythonInvalidResponse = errors.New("platform: invalid python response")

	// ErrInvalidPrediction means a prediction request or durable record failed
	// the Go-owned lineage and state validation.
	ErrInvalidPrediction = errors.New("platform: invalid prediction")

	// ErrPredictionConflict means an immutable prediction identity was reused
	// with a different result.
	ErrPredictionConflict = errors.New("platform: prediction conflict")

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
