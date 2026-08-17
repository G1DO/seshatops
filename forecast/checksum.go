package forecast

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

// Checksum returns the protocol SHA-256 hex of ds. The empty dataset hashes
// the empty byte sequence.
func Checksum(ds Dataset) string {
	examples := append([]Example(nil), ds.Examples...)
	sort.Slice(examples, func(i, j int) bool {
		return checksumLess(examples[i], examples[j])
	})
	var b strings.Builder
	for _, e := range examples {
		b.WriteString(strings.ToLower(e.RowID))
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(e.TenantID))
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(e.ItemID))
		b.WriteByte('\t')
		b.WriteString(e.AsOfDate)
		b.WriteByte('\t')
		b.WriteString(formatInt(int64(e.Label)))
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(e.Split))
		b.WriteByte('\t')
		b.WriteString(e.SourceCutoffDate)
		b.WriteByte('\t')
		b.WriteString(strings.ToLower(e.HistoryHash))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func checksumLess(a, b Example) bool {
	at, bt := strings.ToLower(a.TenantID), strings.ToLower(b.TenantID)
	if at != bt {
		return at < bt
	}
	ai, bi := strings.ToLower(a.ItemID), strings.ToLower(b.ItemID)
	if ai != bi {
		return ai < bi
	}
	return a.AsOfDate < b.AsOfDate
}

func formatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
