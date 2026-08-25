# Getting started

This is a local, disposable reproduction of the public Northstar Foods
scenario. It uses only the synthetic fixture committed under `northstar/`; it
does not import production data and it does not make a production deployment
claim.

## One-command local stack

Prerequisites are Docker Engine and Docker Compose v2. Release-demonstration
commands additionally require host Python 3.10 or newer, Git, and a Git
checkout of this repository. The committed Compose file uses pinned
PostgreSQL, Redpanda, and mock OIDC images plus local-only credentials. Those
values are disposable test configuration, not secrets.

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

After authenticating in the browser, `GET /metrics` is available through the
same-origin Vite proxy to the explicitly authorized local release observer.
It exposes bounded aggregate release signals only; see
[Release observability](operations/observability.md) for access, metric
lifecycles, redaction rules, and practical queries.

## Release demonstration campaign

The release harness runs the real packaged Go runtime, public REST/SSE and OIDC
paths, PostgreSQL, and Redpanda. Each scenario starts from a fresh disposable
Compose environment and removes that environment and its volumes afterward.
Do not point this command at data that must be retained.

Set the exact local-demonstration confirmation, then run the complete campaign
or one named scenario from the repository root:

```bash
export SESHATOPS_DEMO_CONFIRM='I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO'
./scripts/local-stack.sh demo all
./scripts/local-stack.sh demo duplicate-delivery
```

The accepted scenario names are:

| Scenario | Demonstrated local behavior |
| --- | --- |
| `normal-flow` | ERP source transaction through outbox, Redpanda, inbox/projection, REST, and SSE, with matching committed identity and telemetry. |
| `duplicate-delivery` | Exact broker redelivery is an expected `duplicate_noop` and does not duplicate inventory or lineage effects. |
| `poison-isolation` | An incompatible event is quarantined while unrelated valid source work continues. |
| `broker-recovery` | A stopped broker causes visible degradation and durable outbox backlog; restarting the broker and packaged runtime restores readiness and drains the backlog. |
| `deterministic-rebuild` | An authorized tenant rebuild preserves the before/result/after inventory and lineage checksums. |
| `tenant-isolation` | Authenticated cross-tenant read and mutation return minimal `403` responses without payload leakage or source/projection effects; the privileged denial remains audited. |
| `forecast-source-states` | Empty, durable-unapplied, and malformed retained source history report insufficient, stale, and incomplete states without usable rows. |
| `python-degradation` | Python unavailable and timeout outcomes are typed and non-zero, persist no prediction, and do not stop core operations. |

Any failed invariant, timeout, unexpected status, checksum mismatch, missing
telemetry, guard failure, or cleanup failure makes the command exit non-zero.
The `all` campaign runs the scenarios in the order above and stops after the
first failed scenario.

### Evidence, cleanup, and deterministic comparison

By default, evidence is written below `.release-evidence/<UTC timestamp>/`,
which is ignored by Git. Pass `--evidence-dir` to select a new or empty
directory. Repository-local evidence must remain below `.release-evidence`;
paths outside the repository are also accepted. Each attempted scenario writes
a bounded machine-readable `<scenario>.json` result and a derived
`<scenario>.txt` human summary; the terminal also prints a short summary. An
`all` invocation additionally writes
`campaign.json`, including when it stops after a failed scenario. Results
include release commit/version, source fingerprint, and dirty state,
fixture and harness versions, timestamps, bounded/redacted actions,
expectations, observed statuses/counts/durations/checksums/telemetry,
deterministic identity, cleanup status, failure detail, and known limitations.

After a guarded scenario failure, the harness attempts to retain a bounded
Compose status snapshot and recent runtime/Redpanda log tail before cleanup.
It then runs `down --volumes --remove-orphans` against the declared project.
Failed cleanup is itself a failed result. If the target guard has not
succeeded, destructive cleanup is not attempted.

To compare two complete runs, keep the checkout unchanged and use separate
evidence directories:

```bash
./scripts/local-stack.sh demo all \
  --evidence-dir .release-evidence/m5-run-1
./scripts/local-stack.sh demo all \
  --evidence-dir .release-evidence/m5-run-2 \
  --compare .release-evidence/m5-run-1/campaign.json
```

Comparison requires clean checkouts with the same complete release identity,
fixture version, and exact per-scenario deterministic identities. Runtime
timestamps and diagnostic durations remain in each result but are deliberately
excluded from those identities.

To verify that the harness itself propagates a broken expectation, use the
verification-only hook with a real expectation name and require a non-zero
exit:

```bash
./scripts/local-stack.sh demo normal-flow \
  --evidence-dir .release-evidence/m5-forced-failure \
  --break-expectation normal_runtime_ready
```

`--break-expectation` changes only result verification; it is not a public
fault-injection API or evidence that the observed runtime condition actually
failed.

### Local-only fault boundary

Before any reset or fault, the harness renders and validates the repository's
fixed `seshatops-local` project and `compose.yaml`. It requires exactly the
declared services, `SESHATOPS_LOCAL_STACK=true`, the disposable PostgreSQL
service/database/port with no connection-target overrides, `redpanda:9092`, no
host ports on PostgreSQL, Redpanda, or the Go runtime, a selected local Unix
Docker endpoint with `DOCKER_HOST` and `DOCKER_CONTEXT` unset, project-scoped
non-external data volumes and network, fixed service mounts/resolution, and the
exact `SESHATOPS_DEMO_CONFIRM` value above. Every later Compose action pins the
validated Unix endpoint and a private snapshot of the validated Compose file
instead of re-resolving mutable target configuration. The snapshot is removed
after environment cleanup.

Fixture actions are an allowlisted packaged `demo-fixture` command invoked
inside the runtime container. The harness supplies its separate
`SESHATOPS_DEMO_FIXTURE_CONFIRM=I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO` guard only
for those actions. Broker interruption is limited to the declared Redpanda
service, and Python degradation uses fixed packaged command-process boundaries.
None of these controls is reachable from the normal public API, and the
harness does not directly edit a completed projection to manufacture an
outcome.

## Recovery runbooks

If a local run shows degraded readiness, quarantine, or forecast staleness, follow the release runbooks that use only implemented commands:

* [Broker interruption and outbox backlog recovery](operations/runbooks/broker-interruption.md)
* [Poison and incompatible event isolation](operations/runbooks/poison-isolation.md)
* [Tenant-scoped projection rebuild](operations/runbooks/rebuild-checksum.md)
* [Forecast degradation](operations/runbooks/forecast-degradation.md)

These distinguish retryable transport failure, quarantined poison, immutable conflict, authorization denial, and forecast-only degradation, and they never prescribe replay/release when the state is not safely releasable. See the [operations index](operations/README.md) and [release observability](operations/observability.md) for signals and lifecycles.

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
