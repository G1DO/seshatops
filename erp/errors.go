package erp

import "errors"

// Domain and persistence sentinel errors for the Event Spine source transaction.
var (
	// ErrInvalidQuantity is returned when the ordered quantity is not a
	// positive integer within the Event Spine safe range.
	ErrInvalidQuantity = errors.New("erp: invalid quantity")

	// ErrUnknownItem is returned when no inventory row exists for the
	// requested tenant and item.
	ErrUnknownItem = errors.New("erp: unknown inventory item")

	// ErrUnknownSource is returned when a required parent lineage row is
	// missing for the command tenant.
	ErrUnknownSource = errors.New("erp: unknown lineage source")

	// ErrTenantMismatch is returned when the order tenant/context does not
	// match the authoritative inventory row or required identity rules.
	ErrTenantMismatch = errors.New("erp: tenant or context mismatch")

	// ErrInvalidTransition is returned when applying the hop would violate
	// an Event Spine source invariant (for example, negative on-hand
	// quantity or a causation_id that does not match the required parent).
	ErrInvalidTransition = errors.New("erp: invalid inventory transition")

	// ErrDuplicateOrder is returned when the order_id is already accepted.
	ErrDuplicateOrder = errors.New("erp: duplicate order")

	// ErrDuplicateSource is returned when a lineage source row already exists
	// or a 1:1 parent relationship is already used.
	ErrDuplicateSource = errors.New("erp: duplicate lineage source")

	// ErrDuplicateEvent is returned when the outbox already holds event_id.
	ErrDuplicateEvent = errors.New("erp: duplicate outbox event")

	// ErrInvalidFixture is returned when a Northstar fixture envelope is not
	// the expected family for the helper.
	ErrInvalidFixture = errors.New("erp: fixture event family mismatch")
)
