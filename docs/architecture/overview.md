# Architecture

Packages below are the as-built Event Spine, Identity HTTP, M4 stockout
evaluation library, Go-owned forecast feature read boundary, and offline
candidate artifact contract. The `cmd/seshatops` process composes the packages,
applies all three PostgreSQL migrations before serving, and owns the HTTP,
outbox-relay, and Redpanda-consumer lifecycles. Tests also compose
`http.Handler` values and call `relay.DrainOnce` / `platform.ConsumeOnce`.

Event Spine lives in `event/`, `northstar/`, `erp/`, `relay/`, `platform/`,
`api/`, `web/`. Identity lives in `identity/` plus authorized HTTP. Stockout
evaluation and the pure raw feature builder live in `forecast/`; the
tenant-scoped PostgreSQL replay boundary lives in `platform/` and its HTTP
route/authentication lives in `api/`. Python is an offline artifact producer
only; it is not a deployed runtime or database principal. Public demo uses
**Northstar Foods**. **Ahoy is excluded**
([CLEAN_ROOM.md](../CLEAN_ROOM.md)).

```mermaid
flowchart TB
  RT[cmd/seshatops runtime]
  UI[TypeScriptOperationsView]
  ID[GoIdentity]
  API[GoAPI]
  PL[GoPlatform]
  FS[GoForecastFeatureBoundary]
  REL[GoRelay]
  ERP[SyntheticERP]
  BUS[Redpanda]
  PG[PostgreSQL]
  PY[PythonCandidateArtifactProducer]
  RT --> ID
  RT --> API
  RT --> REL
  RT --> PL
  UI <-->|"REST SSE cookies"| API
  UI <-->|"OIDC session"| ID
  API --> ID
  API --> PL
  API --> FS
  FS --> PG
  FS --> PY
  ERP -->|atomic source plus outbox| PG
  REL -->|exact outbox bytes| BUS
  BUS --> PL
  PL --> PG
  ID --> PG
  API --> PG
```

`cmd/seshatops` validates the database URL, broker seeds, OIDC endpoints,
cookie transport, and listen address before opening clients. It exposes
`GET /livez` and `GET /readyz`; readiness stays unavailable until migrations,
broker probes, and both worker loops are healthy. The process handles
`SIGINT`/`SIGTERM` by stopping HTTP acceptance, cancelling the relay and
consumer loops, closing their broker clients, clearing the post-commit
projection notifier, and closing PostgreSQL. Worker failures use bounded
process retry backoff in addition to the existing outbox and broker semantics.

The executable reads `SESHATOPS_LISTEN_ADDR`, `SESHATOPS_DATABASE_URL`,
`SESHATOPS_BROKER_SEEDS`, `SESHATOPS_OIDC_ISSUER`,
`SESHATOPS_OIDC_CLIENT_ID`, `SESHATOPS_OIDC_REDIRECT_URL`,
`SESHATOPS_COOKIE_NAME`, and `SESHATOPS_COOKIE_SECURE`. The OIDC client secret,
audience, session TTL, and optional process-local assignments are configured by
`SESHATOPS_OIDC_CLIENT_SECRET`, `SESHATOPS_OIDC_AUDIENCE`,
`SESHATOPS_SESSION_TTL`, and `SESHATOPS_AUTH_ASSIGNMENTS` respectively.
Assignments use comma-separated `principal|tenant|role` rows; an omitted
assignment list leaves the policy default-deny.

| Component | Language | Owns | Must not own |
| --- | --- | --- | --- |
| `web/` | TypeScript | Presentation, REST/SSE clients | Authorization, datastore, broker |
| `cmd/seshatops` | Go | Process startup, migrations, health, HTTP and worker lifecycle | Domain rules, authorization decisions, event transformation |
| `identity/` | Go | OIDC Auth Code + PKCE, session, allow-list, audit | UI, IdP vendor as a runtime claim |
| `api/` | Go | HTTP/SSE, default-deny, privileged POSTs | Broker publication |
| `platform/` | Go | Inbox, inventory and lineage projection, read-only forecast source replay, rebuild | Outbox publication, authentication, feature persistence |
| `relay/` | Go | Publish exact outbox bytes to Redpanda | Inbox, authorization |
| `erp/` | Go | Lineage hops, order accept, inventory, immutable outbox | SeshatOps policy |
| `event/`, `northstar/` | Go | JSON/JCS envelope, Northstar fixture | Transport or HTTP |
| `forecast/` | Go | Frozen stockout target, temporal dataset, pure raw feature snapshots, typed artifact validation, evaluation metrics, promotion comparison | Transactional state, HTTP, labels in feature rows, Python runtime, model serving |
| `forecast_candidate/` | Python | Produce versioned prediction artifacts from serialized read-only dataset/features | PostgreSQL credentials, writes, authorization, workflow transitions, promotion decisions |

Go module: `github.com/G1DO/seshatops`. Privileged operator POSTs (quarantine
release, replay, rebuild) are in [authorization.md](../security/authorization.md).
There is no deployed Python runtime, object storage, or HTTP ERP command surface. Package `erp`
accepts `RegisterSupplier`, `ReceiveIngredientLot`, `ProduceProductionBatch`,
`DispatchShipment`, and `AcceptOrder`. The TypeScript UI cannot authorize those
writes. `GET /v1/tenants/{tenant_id}/forecast/features` is a read-only Go
boundary over retained `erp.outbox`, `platform.inbox`, and
`platform.inventory_projection` rows; it never stores feature rows or
snapshots and never calls a projection mutator.

| Store | Responsibility |
| --- | --- |
| PostgreSQL | ERP source/outbox, platform inbox/projection, identity audit |
| Redpanda | At-least-once transport. Not the system of record |

OIDC sessions and configured authorization assignments are process-local
(`identity.Store` and `identity.Directory`).

| Path | Rule |
| --- | --- |
| Browser ↔ Go | `/auth/*` and `/v1/*` only |
| ERP → PostgreSQL | Source and outbox in one transaction |
| Relay → Redpanda | Exact stored outbox bytes after commit |
| Browser → PostgreSQL or Redpanda | Prohibited |

Wire and pins: [event-spine.md](../design/specifications/event-spine.md).
Stockout evaluation: [stockout-evaluation.md](../design/specifications/stockout-evaluation.md).
Forecast feature snapshots: [forecast-feature-snapshots.md](../design/specifications/forecast-feature-snapshots.md).
`/v1` HTTP/SSE: [openapi-projection.yaml](../api/openapi-projection.yaml).
`/auth` and allow-list: [authorization.md](../security/authorization.md).
