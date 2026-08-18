// Package forecast implements the frozen M4 stockout evaluation protocol
// from docs/design/specifications/stockout-evaluation.md: deterministic
// Northstar daily on-hand history, chronological labeled dataset construction,
// dataset checksums, and abstention-aware metrics/promotion rules.
//
// It does not read transactional state, train models, speak HTTP, or mutate
// ERP/platform/identity state. Its raw feature snapshot builder is pure and
// exposes no evaluation labels.
package forecast
