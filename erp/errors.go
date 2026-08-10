package erp

import "errors"

// Domain and persistence sentinel errors for the M1 source transaction.
var (
	// ErrInvalidQuantity is returned when the ordered quantity is not a
	// positive integer within the M1 safe range.
	ErrInvalidQuantity = errors.New("erp: invalid quantity")

	// ErrUnknownItem is returned when no inventory row exists for the
	// requested tenant and item.
	ErrUnknownItem = errors.New("erp: unknown inventory item")

	// ErrTenantMismatch is returned when the order tenant/context does not
	// match the authoritative inventory row or required identity rules.
	ErrTenantMismatch = errors.New("erp: tenant or context mismatch")

	// ErrInvalidTransition is returned when applying the order would violate
	// the M1 inventory invariant (for example, negative on-hand quantity).
	ErrInvalidTransition = errors.New("erp: invalid inventory transition")

	// ErrDuplicateOrder is returned when the order_id is already accepted.
	ErrDuplicateOrder = errors.New("erp: duplicate order")

	// ErrDuplicateEvent is returned when the outbox already holds event_id.
	ErrDuplicateEvent = errors.New("erp: duplicate outbox event")
)
