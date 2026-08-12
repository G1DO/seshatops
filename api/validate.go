package api

import (
	"regexp"
	"strings"
)

// Lowercase UUIDv4 with RFC 4122 variant bits (aligned with event package).
var lowerUUIDv4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func validTenantID(tenantID string) bool {
	if tenantID == "" {
		return false
	}
	if tenantID != strings.ToLower(tenantID) {
		return false
	}
	return lowerUUIDv4.MatchString(tenantID)
}
