package event

import (
	"fmt"
	"unicode/utf8"
)

// ensureJCSCompatibleJSON rejects non-UTF-8 input and unpaired UTF-16
// surrogate escapes before encoding/json can rewrite them to U+FFFD.
func ensureJCSCompatibleJSON(raw []byte) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("%w: input is not valid UTF-8", ErrMalformed)
	}
	return rejectUnpairedSurrogateEscapes(raw)
}

// rejectUnpairedSurrogateEscapes scans JSON string literals for \uXXXX
// escapes that form lone UTF-16 surrogates (RFC 8785 incompatible).
func rejectUnpairedSurrogateEscapes(raw []byte) error {
	inString := false
	for i := 0; i < len(raw); {
		c := raw[i]
		if !inString {
			if c == '"' {
				inString = true
			}
			i++
			continue
		}

		if c == '\\' {
			if i+1 >= len(raw) {
				return fmt.Errorf("%w: incomplete JSON string escape", ErrMalformed)
			}
			esc := raw[i+1]
			if esc == 'u' || esc == 'U' {
				unit, next, err := parseJSONUnicodeEscape(raw, i)
				if err != nil {
					return err
				}
				switch {
				case isHighSurrogate(unit):
					if next+1 >= len(raw) || raw[next] != '\\' || (raw[next+1] != 'u' && raw[next+1] != 'U') {
						return fmt.Errorf("%w: unpaired UTF-16 surrogate escape", ErrMalformed)
					}
					low, afterLow, err := parseJSONUnicodeEscape(raw, next)
					if err != nil {
						return err
					}
					if !isLowSurrogate(low) {
						return fmt.Errorf("%w: unpaired UTF-16 surrogate escape", ErrMalformed)
					}
					i = afterLow
				case isLowSurrogate(unit):
					return fmt.Errorf("%w: unpaired UTF-16 surrogate escape", ErrMalformed)
				default:
					i = next
				}
				continue
			}
			i += 2
			continue
		}

		if c == '"' {
			inString = false
		}
		i++
	}
	return nil
}

func parseJSONUnicodeEscape(raw []byte, atSlash int) (unit uint16, next int, err error) {
	// atSlash points at '\' of \uXXXX
	if atSlash+6 > len(raw) {
		return 0, 0, fmt.Errorf("%w: incomplete JSON unicode escape", ErrMalformed)
	}
	if raw[atSlash] != '\\' || (raw[atSlash+1] != 'u' && raw[atSlash+1] != 'U') {
		return 0, 0, fmt.Errorf("%w: invalid JSON unicode escape", ErrMalformed)
	}
	var v uint16
	for _, b := range raw[atSlash+2 : atSlash+6] {
		var nibble byte
		switch {
		case b >= '0' && b <= '9':
			nibble = b - '0'
		case b >= 'a' && b <= 'f':
			nibble = b - 'a' + 10
		case b >= 'A' && b <= 'F':
			nibble = b - 'A' + 10
		default:
			return 0, 0, fmt.Errorf("%w: invalid JSON unicode escape", ErrMalformed)
		}
		v = v<<4 | uint16(nibble)
	}
	return v, atSlash + 6, nil
}

func isHighSurrogate(u uint16) bool { return u >= 0xd800 && u <= 0xdbff }
func isLowSurrogate(u uint16) bool  { return u >= 0xdc00 && u <= 0xdfff }
