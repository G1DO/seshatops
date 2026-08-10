# Synthetic Data Provenance — Northstar Foods M1 Order Line

This record covers the deterministic M1 fixture and golden event vectors
introduced for Issue #22, and the Issue #23 inventory seed that consumes the
same fixture. It does not claim broker transport, projection, or production
readiness.

## Provenance fields

| Field | Value |
| --- | --- |
| Origin | Created for SeshatOps / fictional Northstar Foods public scenario |
| Generation method | Fixed Go generator `northstar.Generate` with declared seed `northstar-m1-order-line-v1`; identifiers and quantities match the illustrative example in [CONTRACTS.md](../../CONTRACTS.md). Issue #23 seeds `erp.inventory_items` from that fixture via `erp.SeedNorthstarInventory`. |
| License | Redistributed under the repository [Apache License 2.0](../../LICENSE) |
| Reproducibility | Run `go test ./northstar ./event ./erp` from the repository root (PostgreSQL via Testcontainers using `erp.PostgresImage`, or `SESHATOPS_TEST_DATABASE_URL`). The same seed must emit identical canonical JCS bytes and SHA-256 content hash as `northstar/testdata/order_line_v1.jcs` and `northstar/testdata/order_line_v1.sha256`, and `erp.AcceptOrder` must persist those exact outbox bytes for the fixture command. |
| Independence | Not derived from private production data, private schemas, or Ahoy material |

## Regeneration instructions

1. Use Go `1.25.0` as pinned in `go.mod` / [CONTRACTS.md](../../CONTRACTS.md) §9.
2. From the repository root, run:

   ```text
   go test ./event ./northstar ./erp -count=1 -timeout 15m
   ```

3. Expected results:
   - `event` package accepts the valid v1 envelope and rejects declared incompatible cases.
   - `northstar.Generate(northstar.DefaultSeed)` twice yields the same logical event and content hash.
   - Empty or other seeds are rejected; only the declared default seed is supported.
   - Golden files under `event/testdata/` and `northstar/testdata/` remain byte-stable.
   - `erp` accepts the fixture order against seeded inventory and stores matching outbox bytes.

Do not regenerate goldens by capturing wall-clock time or random IDs. The M1
fixture is intentionally fixed to the declared seed and CONTRACTS.md example
values.

## Clean-room note

All committed identifiers (`item-flour-001`, synthetic UUIDs, Northstar Foods
naming) are fictional public-scenario material. No private hosts, account IDs,
or Ahoy-derived business rules are included.
