package identity

import "errors"

// ErrUnauthenticated is returned when no fresh Go-owned session exists.
var ErrUnauthenticated = errors.New("identity: unauthenticated")

// ErrForbidden is returned when a fresh session is not authorized for the
// requested tenant, resource, and action.
var ErrForbidden = errors.New("identity: forbidden")
