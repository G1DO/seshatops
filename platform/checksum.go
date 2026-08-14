package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ChecksumTenant returns the docs/design/specifications/event-spine.md §8 SHA-256 hex checksum of one
// tenant's complete inventory projection at the current committed snapshot.
// The empty projection hashes the empty byte sequence.
func ChecksumTenant(ctx context.Context, db *sql.DB, tenantID string) (string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT tenant_id, item_id, quantity_on_hand, aggregate_version
		FROM platform.inventory_projection
		WHERE tenant_id = $1
	`, strings.ToLower(tenantID))
	if err != nil {
		return "", fmt.Errorf("platform: checksum query: %w", err)
	}
	defer rows.Close()

	type row struct {
		tenantID string
		itemID   string
		qty      int64
		version  int64
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.tenantID, &r.itemID, &r.qty, &r.version); err != nil {
			return "", fmt.Errorf("platform: checksum scan: %w", err)
		}
		list = append(list, row{
			tenantID: strings.ToLower(r.tenantID),
			itemID:   strings.ToLower(r.itemID),
			qty:      r.qty,
			version:  r.version,
		})
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("platform: checksum rows: %w", err)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].tenantID != list[j].tenantID {
			return list[i].tenantID < list[j].tenantID
		}
		return list[i].itemID < list[j].itemID
	})

	var b strings.Builder
	for _, r := range list {
		b.WriteString(r.tenantID)
		b.WriteByte('\t')
		b.WriteString(r.itemID)
		b.WriteByte('\t')
		b.WriteString(formatInt(r.qty))
		b.WriteByte('\t')
		b.WriteString(formatInt(r.version))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), nil
}

func formatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
