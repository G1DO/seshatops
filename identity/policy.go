package identity

// Authorizer evaluates allow-list decisions for a Go-selected tenant or
// release scope. Callers must treat a nil Authorizer as deny.
type Authorizer interface {
	Allow(principalID, scopeID, resource, action string) error
}

// Policy binds platform assignments to the frozen permission matrix.
// Client-supplied tenant or role fields are not inputs.
type Policy struct {
	dir *Directory
}

// NewPolicy returns an Authorizer over dir. A nil directory fails closed.
func NewPolicy(dir *Directory) *Policy {
	return &Policy{dir: dir}
}

// Allow reports whether principalID may perform action on resource in scopeID.
// For product APIs scopeID is the path tenant UUID. The aggregate release
// metrics handler selects ScopeRuntime itself; no caller-controlled request
// field can select that scope. Missing, incomplete, or unmatched membership is
// deny.
func (p *Policy) Allow(principalID, scopeID, resource, action string) error {
	if p == nil || p.dir == nil {
		return ErrForbidden
	}
	if principalID == "" || scopeID == "" || resource == "" || action == "" {
		return ErrForbidden
	}

	rows := p.dir.Assignments(principalID)
	if len(rows) == 0 {
		return ErrForbidden
	}

	var roles []string
	matchedScope := false
	for _, row := range rows {
		if row.PrincipalID != principalID {
			return ErrForbidden
		}
		if row.TenantID == "" {
			return ErrForbidden
		}
		if row.TenantID != scopeID {
			continue
		}
		if row.RoleID == "" {
			return ErrForbidden
		}
		matchedScope = true
		roles = append(roles, row.RoleID)
	}
	if !matchedScope {
		return ErrForbidden
	}
	for _, role := range roles {
		if matrixAllows(scopeID, role, resource, action) {
			return nil
		}
	}
	return ErrForbidden
}
