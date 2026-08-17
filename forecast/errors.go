package forecast

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel failures for the frozen M4 stockout evaluation protocol.
var (
	// ErrUnsupportedSeed is returned when GenerateHistory is called with an
	// empty or unknown seed.
	ErrUnsupportedSeed = errors.New("forecast: unsupported seed")

	// ErrInvalidInput is returned when dataset construction or evaluation
	// receives malformed observations, tenant, split, or predictions.
	ErrInvalidInput = errors.New("forecast: invalid input")
)

func wrapInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, args...))
}

func unsupportedSeed(seed string) error {
	if strings.TrimSpace(seed) == "" {
		return fmt.Errorf("%w: empty seed", ErrUnsupportedSeed)
	}
	return fmt.Errorf("%w: %s", ErrUnsupportedSeed, seed)
}
