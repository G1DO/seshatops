package identity

import (
	"errors"
	"testing"
)

func TestPolicyAllowsSameTenantInventoryRead(t *testing.T) {
	p := NewPolicy(NewDirectory(Assignment{
		PrincipalID: "operator-northstar",
		TenantID:    TenantNS001UUID,
		RoleID:      RoleOpsReader,
	}))
	if err := p.Allow("operator-northstar", TenantNS001UUID, ResInventoryProjection, ActRead); err != nil {
		t.Fatalf("MX-001 allow: %v", err)
	}
	if err := p.Allow("operator-northstar", TenantNS001UUID, ResOpsVisibility, ActRead); err != nil {
		t.Fatalf("MX-002 allow: %v", err)
	}
}

func TestPolicyDeniesCrossTenantInventoryRead(t *testing.T) {
	p := NewPolicy(NewDirectory(Assignment{
		PrincipalID: "operator-northstar",
		TenantID:    TenantNS001UUID,
		RoleID:      RoleOpsReader,
	}))
	if err := p.Allow("operator-northstar", TenantNS002UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant err=%v", err)
	}
}

func TestPolicyDeniesPlatformOperatorInventoryRead(t *testing.T) {
	p := NewPolicy(NewDirectory(Assignment{
		PrincipalID: "platform-operator",
		TenantID:    TenantNS001UUID,
		RoleID:      RolePlatformOperator,
	}))
	if err := p.Allow("platform-operator", TenantNS001UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator inventory err=%v", err)
	}
	if err := p.Allow("platform-operator", TenantNS001UUID, ResOpsVisibility, ActRead); err != nil {
		t.Fatalf("MX-003 allow: %v", err)
	}
}

func TestPolicyDeniesUnassignedAndServicePrincipal(t *testing.T) {
	p := NewPolicy(NewDirectory())
	if err := p.Allow("svc-relay", TenantNS001UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unassigned err=%v", err)
	}
}

func TestPolicyDeniesMissingOrAmbiguousMembership(t *testing.T) {
	t.Run("empty_role", func(t *testing.T) {
		p := NewPolicy(NewDirectory(Assignment{
			PrincipalID: "reader",
			TenantID:    TenantNS001UUID,
			RoleID:      "",
		}))
		if err := p.Allow("reader", TenantNS001UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
			t.Fatalf("empty role err=%v", err)
		}
	})
	t.Run("empty_tenant", func(t *testing.T) {
		p := NewPolicy(NewDirectory(Assignment{
			PrincipalID: "reader",
			TenantID:    "",
			RoleID:      RoleOpsReader,
		}))
		if err := p.Allow("reader", TenantNS001UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
			t.Fatalf("empty tenant err=%v", err)
		}
	})
	t.Run("empty_request_fields", func(t *testing.T) {
		p := NewPolicy(NewDirectory(Assignment{
			PrincipalID: "reader",
			TenantID:    TenantNS001UUID,
			RoleID:      RoleOpsReader,
		}))
		if err := p.Allow("", TenantNS001UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
			t.Fatalf("empty principal err=%v", err)
		}
		if err := p.Allow("reader", "", ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
			t.Fatalf("empty tenant assertion err=%v", err)
		}
	})
	t.Run("nil_policy", func(t *testing.T) {
		var p *Policy
		if err := p.Allow("reader", TenantNS001UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
			t.Fatalf("nil policy err=%v", err)
		}
		if err := NewPolicy(nil).Allow("reader", TenantNS001UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
			t.Fatalf("nil directory err=%v", err)
		}
	})
}

func TestPolicyDeniesCrossTenantOpsVisibility(t *testing.T) {
	p := NewPolicy(NewDirectory(Assignment{
		PrincipalID: "operator-northstar",
		TenantID:    TenantNS001UUID,
		RoleID:      RoleOpsReader,
	}))
	if err := p.Allow("operator-northstar", TenantNS002UUID, ResOpsVisibility, ActRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant ops err=%v", err)
	}
}

func TestPolicyAllowsPlatformOperatorPrivilegedOps(t *testing.T) {
	p := NewPolicy(NewDirectory(Assignment{
		PrincipalID: "platform-operator",
		TenantID:    TenantNS001UUID,
		RoleID:      RolePlatformOperator,
	}))
	if err := p.Allow("platform-operator", TenantNS001UUID, ResQuarantine, ActQuarantineRelease); err != nil {
		t.Fatalf("MX-004 allow: %v", err)
	}
	if err := p.Allow("platform-operator", TenantNS001UUID, ResReplay, ActReplay); err != nil {
		t.Fatalf("MX-005 allow: %v", err)
	}
	if err := p.Allow("platform-operator", TenantNS001UUID, ResRebuild, ActRebuild); err != nil {
		t.Fatalf("MX-006 allow: %v", err)
	}
	if err := p.Allow("platform-operator", TenantNS001UUID, ResAudit, ActAuditRead); err != nil {
		t.Fatalf("MX-007 allow: %v", err)
	}
}

func TestPolicyDeniesPrivilegedActionsForOpsReader(t *testing.T) {
	p := NewPolicy(NewDirectory(Assignment{
		PrincipalID: "operator-northstar",
		TenantID:    TenantNS001UUID,
		RoleID:      RoleOpsReader,
	}))
	if err := p.Allow("operator-northstar", TenantNS001UUID, ResQuarantine, ActQuarantineRelease); !errors.Is(err, ErrForbidden) {
		t.Fatalf("release err=%v", err)
	}
	if err := p.Allow("operator-northstar", TenantNS001UUID, ResReplay, ActReplay); !errors.Is(err, ErrForbidden) {
		t.Fatalf("replay err=%v", err)
	}
	if err := p.Allow("operator-northstar", TenantNS001UUID, ResRebuild, ActRebuild); !errors.Is(err, ErrForbidden) {
		t.Fatalf("rebuild err=%v", err)
	}
	if err := p.Allow("operator-northstar", TenantNS001UUID, ResAudit, ActAuditRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("audit read err=%v", err)
	}
}

func TestPolicyDeniesCrossTenantPrivilegedOps(t *testing.T) {
	p := NewPolicy(NewDirectory(Assignment{
		PrincipalID: "platform-operator",
		TenantID:    TenantNS001UUID,
		RoleID:      RolePlatformOperator,
	}))
	if err := p.Allow("platform-operator", TenantNS002UUID, ResQuarantine, ActQuarantineRelease); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant release err=%v", err)
	}
	if err := p.Allow("platform-operator", TenantNS002UUID, ResReplay, ActReplay); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant replay err=%v", err)
	}
	if err := p.Allow("platform-operator", TenantNS002UUID, ResRebuild, ActRebuild); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant rebuild err=%v", err)
	}
	if err := p.Allow("platform-operator", TenantNS002UUID, ResAudit, ActAuditRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant audit read err=%v", err)
	}
}

func TestDirectoryClearRevokesMembership(t *testing.T) {
	dir := NewDirectory(Assignment{
		PrincipalID: "operator-northstar",
		TenantID:    TenantNS001UUID,
		RoleID:      RoleOpsReader,
	})
	p := NewPolicy(dir)
	if err := p.Allow("operator-northstar", TenantNS001UUID, ResInventoryProjection, ActRead); err != nil {
		t.Fatal(err)
	}
	dir.Clear("operator-northstar")
	if err := p.Allow("operator-northstar", TenantNS001UUID, ResInventoryProjection, ActRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("after clear err=%v", err)
	}
}
