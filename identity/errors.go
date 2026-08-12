package identity

import "errors"

// ErrUnauthenticated is returned when no fresh Go-owned session exists.
var ErrUnauthenticated = errors.New("identity: unauthenticated")
