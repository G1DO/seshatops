package identity

import "sync"

// Assignment is a platform-owned principal-to-tenant role binding.
// TenantID is the tenant UUID, not an IdP or client-supplied field.
type Assignment struct {
	PrincipalID string
	TenantID    string
	RoleID      string
}

// Directory is an in-memory assignment store. It is process-local and is not
// a durable policy schema or database table.
type Directory struct {
	mu          sync.Mutex
	assignments map[string][]Assignment
}

// NewDirectory constructs a directory from the given assignments. Incomplete
// rows are stored so evaluation can fail closed.
func NewDirectory(as ...Assignment) *Directory {
	d := &Directory{assignments: make(map[string][]Assignment)}
	for _, a := range as {
		d.addLocked(a)
	}
	return d
}

// Assign records a platform membership row.
func (d *Directory) Assign(a Assignment) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.assignments == nil {
		d.assignments = make(map[string][]Assignment)
	}
	d.addLocked(a)
}

// Clear removes every assignment for principalID. Missing principals are ignored.
func (d *Directory) Clear(principalID string) {
	if d == nil || principalID == "" {
		return
	}
	d.mu.Lock()
	delete(d.assignments, principalID)
	d.mu.Unlock()
}

// Assignments returns a copy of the rows bound to principalID.
func (d *Directory) Assignments(principalID string) []Assignment {
	if d == nil || principalID == "" {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	src := d.assignments[principalID]
	if len(src) == 0 {
		return nil
	}
	out := make([]Assignment, len(src))
	copy(out, src)
	return out
}

func (d *Directory) addLocked(a Assignment) {
	if a.PrincipalID == "" {
		return
	}
	d.assignments[a.PrincipalID] = append(d.assignments[a.PrincipalID], a)
}
