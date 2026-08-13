package identity

import (
	"context"
	"errors"
	"testing"
)

func TestAppendDecisionRejectsIncomplete(t *testing.T) {
	ctx := context.Background()
	base := DecisionRecord{
		PrincipalID: "platform-operator",
		TenantID:    TenantNS001UUID,
		Resource:    ResQuarantine,
		Action:      ActQuarantineRelease,
		Outcome:     "allow",
		Reason:      "matrix_allow",
	}
	cases := []struct {
		name string
		rec  DecisionRecord
	}{
		{name: "empty_principal", rec: DecisionRecord{TenantID: base.TenantID, Resource: base.Resource, Action: base.Action, Outcome: base.Outcome, Reason: base.Reason}},
		{name: "empty_tenant", rec: DecisionRecord{PrincipalID: base.PrincipalID, Resource: base.Resource, Action: base.Action, Outcome: base.Outcome, Reason: base.Reason}},
		{name: "empty_resource", rec: DecisionRecord{PrincipalID: base.PrincipalID, TenantID: base.TenantID, Action: base.Action, Outcome: base.Outcome, Reason: base.Reason}},
		{name: "empty_action", rec: DecisionRecord{PrincipalID: base.PrincipalID, TenantID: base.TenantID, Resource: base.Resource, Outcome: base.Outcome, Reason: base.Reason}},
		{name: "empty_reason", rec: DecisionRecord{PrincipalID: base.PrincipalID, TenantID: base.TenantID, Resource: base.Resource, Action: base.Action, Outcome: base.Outcome}},
		{name: "empty_outcome", rec: DecisionRecord{PrincipalID: base.PrincipalID, TenantID: base.TenantID, Resource: base.Resource, Action: base.Action, Reason: base.Reason}},
		{name: "unknown_outcome", rec: DecisionRecord{PrincipalID: base.PrincipalID, TenantID: base.TenantID, Resource: base.Resource, Action: base.Action, Outcome: "maybe", Reason: base.Reason}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := AppendDecision(ctx, nil, tc.rec); !errors.Is(err, ErrInvalidDecision) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestAppendDecisionRejectsNilDBAfterValidFields(t *testing.T) {
	_, err := AppendDecision(context.Background(), nil, DecisionRecord{
		PrincipalID: "platform-operator",
		TenantID:    TenantNS001UUID,
		Resource:    ResAudit,
		Action:      ActAuditRead,
		Outcome:     "deny",
		Reason:      "forbidden",
	})
	if err == nil || errors.Is(err, ErrInvalidDecision) {
		t.Fatalf("err=%v", err)
	}
}

func TestListDecisionsForTenantRejectsEmptyTenant(t *testing.T) {
	if _, err := ListDecisionsForTenant(context.Background(), nil, ""); !errors.Is(err, ErrInvalidDecision) {
		t.Fatalf("err=%v", err)
	}
}
