package identity

// Authorizer evaluates tenant-scoped allow-list decisions. Callers must treat
// a nil Authorizer as deny.
type Authorizer interface {
	Allow(principalID, tenantID, resource, action string) error
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

// Allow reports whether principalID may perform action on resource in tenantID.
// tenantID is the path assertion (tenant UUID). Missing, incomplete, or
// unmatched membership is deny.
func (p *Policy) Allow(principalID, tenantID, resource, action string) error {
	if p == nil || p.dir == nil {
		return ErrForbidden
	}
	if principalID == "" || tenantID == "" || resource == "" || action == "" {
		return ErrForbidden
	}

	rows := p.dir.Assignments(principalID)
	if len(rows) == 0 {
		return ErrForbidden
	}

	var roles []string
	matchedTenant := false
	for _, row := range rows {
		if row.PrincipalID != principalID {
			return ErrForbidden
		}
		if row.TenantID == "" {
			return ErrForbidden
		}
		if row.TenantID != tenantID {
			continue
		}
		if row.RoleID == "" {
			return ErrForbidden
		}
		matchedTenant = true
		roles = append(roles, row.RoleID)
	}
	if !matchedTenant {
		return ErrForbidden
	}
	for _, role := range roles {
		if matrixAllows(tenantID, role, resource, action) {
			return nil
		}
	}
	return ErrForbidden
}
