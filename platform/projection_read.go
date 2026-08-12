package platform

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
)

// ProjectionRow is one committed inventory_projection row.
type ProjectionRow struct {
	ItemID           string
	QuantityOnHand   int64
	AggregateVersion int64
}

// ListTenantProjection returns all committed inventory projection rows for a
// tenant, ordered by item_id. Missing tenants yield an empty slice.
func ListTenantProjection(ctx context.Context, db *sql.DB, tenantID string) ([]ProjectionRow, error) {
	if db == nil {
		return nil, fmt.Errorf("platform: nil db")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT item_id, quantity_on_hand, aggregate_version
		FROM platform.inventory_projection
		WHERE tenant_id = $1
		ORDER BY item_id
	`, strings.ToLower(tenantID))
	if err != nil {
		return nil, fmt.Errorf("platform: list projection: %w", err)
	}
	defer rows.Close()

	var out []ProjectionRow
	for rows.Next() {
		var r ProjectionRow
		if err := rows.Scan(&r.ItemID, &r.QuantityOnHand, &r.AggregateVersion); err != nil {
			return nil, fmt.Errorf("platform: list projection scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform: list projection rows: %w", err)
	}
	if out == nil {
		out = []ProjectionRow{}
	}
	return out, nil
}

// AppliedUpdate is a projection-mutating commit notification for Issue #27.
type AppliedUpdate struct {
	TenantID         string
	ItemID           string
	QuantityOnHand   int64
	AggregateVersion int64
	EventID          string
}

// AppliedNotifier receives non-blocking notifications after a projection
// transaction commits with disposition applied.
type AppliedNotifier interface {
	NotifyApplied(update AppliedUpdate)
}

type notifierHolder struct {
	n AppliedNotifier
}

var appliedNotifier atomic.Pointer[notifierHolder]

// SetAppliedNotifier installs the post-commit projection notifier. Pass nil to
// clear. The notifier must not block projection commits.
func SetAppliedNotifier(n AppliedNotifier) {
	if n == nil {
		appliedNotifier.Store(nil)
		return
	}
	appliedNotifier.Store(&notifierHolder{n: n})
}

func notifyApplied(update AppliedUpdate) {
	h := appliedNotifier.Load()
	if h == nil || h.n == nil {
		return
	}
	h.n.NotifyApplied(update)
}

// SetFailBeforeCommitForTest injects a fault before PostgreSQL commit. Pass nil
// to clear. Intended for same-module and api integration tests only.
func SetFailBeforeCommitForTest(fn func(ctx context.Context) error) {
	setTestFailBeforeCommitForTest(fn)
}
