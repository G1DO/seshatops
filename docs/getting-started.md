# Getting started

This is a local, disposable reproduction of the public Northstar Foods
scenario. It uses only the synthetic fixture committed under `northstar/`; it
does not import production data and it does not make a production deployment
claim.

## One-command local stack

Prerequisites are Docker Engine and Docker Compose v2. The committed Compose
file uses pinned PostgreSQL, Redpanda, and mock OIDC images plus local-only
credentials. Those values are disposable test configuration, not secrets.

From the repository root:

```bash
./scripts/local-stack.sh quickstart
```

Open `http://web.seshatops.localhost:5173` and sign in as
`northstar-demo-operator`; the mock OIDC page accepts any password. The flow
uses Authorization Code + PKCE, and the browser sends application requests
only to the Go `/auth/*` and `/v1/*` paths. No session injection or trusted
tenant header is used.

The lifecycle commands are:

```bash
./scripts/local-stack.sh status
./scripts/local-stack.sh logs [service]
./scripts/local-stack.sh down
SESHATOPS_LOCAL_RESET_CONFIRM='I_UNDERSTAND_DISPOSABLE_LOCAL_RESET' \
  ./scripts/local-stack.sh reset
./scripts/local-stack.sh smoke
```

`down` stops the Compose project and preserves its named PostgreSQL and
Redpanda volumes. `reset` removes only that project, its network, and its
volumes, and requires the exact confirmation token shown above. `smoke` starts
the stack from a clean disposable state, verifies the browser login and tenant
authorization flow, restarts the Go runtime, and verifies the bootstrap again.

## Individual commands

### Bootstrap

Start PostgreSQL and Redpanda using the repository's pinned images/configuration,
then set a disposable database URL and broker seed. The bootstrap command only
needs database and broker configuration; it does not require OIDC because it
does not serve HTTP.

```bash
export SESHATOPS_DATABASE_URL='postgres://seshatops:REPLACE_ME@localhost:5432/seshatops_northstar_disposable'
export SESHATOPS_BROKER_SEEDS='localhost:9092'
go run ./cmd/seshatops bootstrap
```

The command applies the ERP, platform, and identity migrations, calls the
existing `erp` source methods in the supplier → lot → batch → shipment → order
path, drains the transactional outbox, and waits up to two minutes by
default for the real projection consumer. On success it emits one JSON object
with the fixture version, event IDs/counts, outbox states, inventory and
lineage checksums, lineage identifiers, and the local authorization
configuration.

The fixture seed is fixed at `northstar-m3-lineage-v1`. A second invocation
verifies matching immutable source/outbox content and returns the same summary
without duplicating source or projection effects. A partial source run can be
re-run to continue from the last committed ERP transaction. Override the
bounded wait with `SESHATOPS_NORTHSTAR_BOOTSTRAP_TIMEOUT=3m` when diagnosing a
slow disposable environment.

### Frozen forecast

After the platform schema is available, run the one-shot frozen M4 forecast
runner from the repository root:

```bash
export SESHATOPS_DATABASE_URL='postgres://seshatops:REPLACE_ME@localhost:5432/seshatops_northstar_disposable'
export SESHATOPS_FORECAST_PYTHON='python3'
export SESHATOPS_FORECAST_CONFIRM='I_UNDERSTAND_FROZEN_M4_FORECAST_WRITE'
go run ./cmd/seshatops forecast
```

The command rebuilds `northstar-m4-stockout-v1` under protocol
`m4-stockout-eval-v1`, invokes `forecast_candidate/stockout_candidate.py`,
evaluates the artifact in Go, and persists the selected advisory result. The
declared frozen outcome selects `seasonal_naive`; the command prints one
bounded JSON result containing lineage, split metrics, selection, prediction
identity/status, and limitations. A second invocation returns the same
immutable prediction identity. Set `SESHATOPS_FORECAST_CANDIDATE` when running
from a different working directory.

Python failures are bounded and fail before prediction persistence. The frozen
M4 source is intentionally separate from the sparse live Event Spine history;
authorized forecast reads therefore report stale, unavailable, or incomplete
freshness when the live source does not match the frozen snapshot.

For the HTTP runtime, configure the same database/broker plus the required
OIDC, cookie, and listen-address variables. The deterministic local operator
assignments are:

```text
northstar-demo-operator|11111111-1111-4111-8111-111111111111|ROLE-OPS-READER
northstar-demo-operator|11111111-1111-4111-8111-111111111111|ROLE-PLATFORM-OPERATOR
```

Set them as the comma-separated `SESHATOPS_AUTH_ASSIGNMENTS` value. There is
intentionally no assignment for `TENANT-NS-002`; the identity matrix also has
no permissive fallback for that tenant.

## Scoped Northstar reset

Reset is a separate explicit command and is never run by bootstrap. It deletes
only the `TENANT-NS-001` ERP source/outbox rows and derived platform rows. It
does not delete append-only identity audit history. The command refuses remote
hosts and any database name other than the exact disposable Northstar name,
and requires an explicit confirmation token:

```bash
export SESHATOPS_NORTHSTAR_RESET_CONFIRM='I_UNDERSTAND_DISPOSABLE_NORTHSTAR_RESET'
go run ./cmd/seshatops reset-northstar
```

Redpanda history is retained. The next bootstrap uses a fresh one-shot
consumer group and therefore replays retained events through the normal
duplicate-safe consumer path.
