package identity

// Stable identifiers from docs/security/PERMISSION_MATRIX.md. Query APIs
// evaluate MX-001 for inventory and MX-002/MX-003 for ops visibility.
// Privileged rows exist so unmatched roles fail closed; they do not
// authorize quarantine/replay HTTP surfaces.
const (
	TenantNS001     = "TENANT-NS-001"
	TenantNS001UUID = "11111111-1111-4111-8111-111111111111"
	TenantNS002     = "TENANT-NS-002"
	TenantNS002UUID = "22222222-2222-4222-8222-222222222222"

	RoleOpsReader        = "ROLE-OPS-READER"
	RolePlatformOperator = "ROLE-PLATFORM-OPERATOR"

	ResInventoryProjection = "RES-INVENTORY-PROJECTION"
	ResOpsVisibility       = "RES-OPS-VISIBILITY"
	ResQuarantine          = "RES-QUARANTINE"
	ResReplay              = "RES-REPLAY"
	ResRebuild             = "RES-REBUILD"
	ResAudit               = "RES-AUDIT"

	ActRead              = "ACT-READ"
	ActQuarantineRelease = "ACT-QUARANTINE-RELEASE"
	ActReplay            = "ACT-REPLAY"
	ActRebuild           = "ACT-REBUILD"
	ActAuditRead         = "ACT-AUDIT-READ"

	MX001 = "MX-001"
	MX002 = "MX-002"
	MX003 = "MX-003"
	MX004 = "MX-004"
	MX005 = "MX-005"
	MX006 = "MX-006"
	MX007 = "MX-007"
)

type allowRow struct {
	ID       string
	TenantID string
	RoleID   string
	Resource string
	Action   string
}

// frozenAllowList is the Issue #44 demo matrix keyed by tenant UUID.
var frozenAllowList = []allowRow{
	{MX001, TenantNS001UUID, RoleOpsReader, ResInventoryProjection, ActRead},
	{MX002, TenantNS001UUID, RoleOpsReader, ResOpsVisibility, ActRead},
	{MX003, TenantNS001UUID, RolePlatformOperator, ResOpsVisibility, ActRead},
	{MX004, TenantNS001UUID, RolePlatformOperator, ResQuarantine, ActQuarantineRelease},
	{MX005, TenantNS001UUID, RolePlatformOperator, ResReplay, ActReplay},
	{MX006, TenantNS001UUID, RolePlatformOperator, ResRebuild, ActRebuild},
	{MX007, TenantNS001UUID, RolePlatformOperator, ResAudit, ActAuditRead},
}

func matrixAllows(tenantID, roleID, resource, action string) bool {
	if tenantID == "" || roleID == "" || resource == "" || action == "" {
		return false
	}
	for _, row := range frozenAllowList {
		if row.TenantID == tenantID && row.RoleID == roleID && row.Resource == resource && row.Action == action {
			return true
		}
	}
	return false
}
