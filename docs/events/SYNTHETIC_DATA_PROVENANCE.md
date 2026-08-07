# Synthetic Data Provenance — Northstar Foods M1 Order Line

This record covers the deterministic M1 fixture and golden event vectors
introduced for Issue #22. It does not claim runtime transport, projection, or
production readiness.

## Provenance fields

| Field | Value |
| --- | --- |
| Origin | Created for SeshatOps / fictional Northstar Foods public scenario |
| Generation method | Fixed Go generator `northstar.Generate` with declared seed `northstar-m1-order-line-v1`; identifiers and quantities match the illustrative example in [CONTRACTS.md](../../CONTRACTS.md) |
| License | Redistributed under the repository [Apache License 2.0](../../LICENSE) |
| Reproducibility | Run `go test ./northstar ./event` from the repository root. The same seed must emit identical canonical JCS bytes and SHA-256 content hash as `northstar/testdata/order_line_v1.jcs` and `northstar/testdata/order_line_v1.sha256` |
| Independence | Not derived from private production data, private schemas, or Ahoy material |

## Regeneration instructions

1. Use Go `1.25.0` as pinned in `go.mod` / [CONTRACTS.md](../../CONTRACTS.md) §9.
2. From the repository root, run:

```text
go test ./event ./northstar
```

3. Expected results:
   - `event` package accepts the valid v1 envelope and rejects declared incompatible cases.
   - `northstar.Generate(northstar.DefaultSeed)` twice yields the same logical event and content hash.
   - Golden files under `event/testdata/` and `northstar/testdata/` remain byte-stable.

Do not regenerate goldens by capturing wall-clock time or random IDs. The M1
fixture is intentionally fixed to the declared seed and CONTRACTS.md example
values.

## Clean-room note

All committed identifiers (`item-flour-001`, synthetic UUIDs, Northstar Foods
naming) are fictional public-scenario material. No private hosts, account IDs,
or Ahoy-derived business rules are included.
