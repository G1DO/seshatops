#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PROJECT_NAME=seshatops-local
COMPOSE=(docker compose --project-name "$PROJECT_NAME" --file "$ROOT_DIR/compose.yaml")
RESET_CONFIRMATION=I_UNDERSTAND_DISPOSABLE_LOCAL_RESET
FORECAST_CONFIRMATION=I_UNDERSTAND_FROZEN_M4_FORECAST_WRITE

compose() {
  "${COMPOSE[@]}" "$@"
}

usage() {
  cat <<'EOF'
Usage: scripts/local-stack.sh <command>

Commands:
  quickstart  Build, start, bootstrap Northstar, and persist the frozen forecast
  status      Show local service status
  logs        Follow service logs (optionally pass service names)
  down        Stop the stack and preserve the disposable database volume
  reset       Destructively remove only this stack and its disposable volumes
  smoke       Run the CI-compatible startup, restart, routing, and OIDC smoke path
  demo        Run all release demonstrations or one named scenario
EOF
}

start() {
  compose up --build --detach --wait --wait-timeout 180
}

run_bootstrap() {
  compose run --rm runtime bootstrap
}

run_forecast() {
  compose run --rm -e "SESHATOPS_FORECAST_CONFIRM=$FORECAST_CONFIRMATION" runtime forecast
}

print_demo_instructions() {
  cat <<'EOF'

SeshatOps is ready at http://web.seshatops.localhost:5173
Select Log in and enter the synthetic local-only identity:
  username: northstar-demo-operator
  password: none (the mock provider has no password)

The browser uses same-origin /auth/* and /v1/* routes. PostgreSQL, Redpanda,
and the Go runtime are not published to the host.
EOF
}

quickstart() {
  start
  run_bootstrap
  run_forecast
  print_demo_instructions
}

status() {
  compose ps
}

logs() {
  if [[ $# -eq 0 ]]; then
    compose logs -f --tail=200
    return
  fi
  compose logs -f --tail=200 "$@"
}

down() {
  compose down --remove-orphans
}

reset() {
  if [[ "${SESHATOPS_LOCAL_RESET_CONFIRM:-}" != "$RESET_CONFIRMATION" ]]; then
    printf 'Refusing reset: set SESHATOPS_LOCAL_RESET_CONFIRM=%s\n' "$RESET_CONFIRMATION" >&2
    return 2
  fi
  compose down --volumes --remove-orphans
}

assert_json_field() {
  local raw=$1
  local expression=$2
  printf '%s\n' "$raw" | compose exec -T runtime python3 -c \
    "import json,sys; value=json.load(sys.stdin); assert $expression, value"
}

smoke() {
  local cleanup_status=0
  cleanup() {
    cleanup_status=$?
    compose down --remove-orphans >/dev/null 2>&1 || true
    return "$cleanup_status"
  }
  trap cleanup EXIT

  start
  local bootstrap_output forecast_output second_bootstrap_output
  bootstrap_output=$(run_bootstrap)
  assert_json_field "$bootstrap_output" "value['status'] == 'complete' and value['event_counts']['source'] == 5"
  forecast_output=$(run_forecast)
  assert_json_field "$forecast_output" "value['prediction_status'] == 'predicted' and value['observability']['python_invocation_outcome'] == 'available' and value['observability']['lifecycle'] == 'process_local_invocation'"

  compose exec -T runtime python3 /app/scripts/local-smoke.py

  compose restart runtime
  compose up --wait --wait-timeout 180
  second_bootstrap_output=$(run_bootstrap)
  assert_json_field "$second_bootstrap_output" "value['status'] == 'complete' and value['event_counts']['source'] == 5"

  printf 'local stack smoke passed: startup, bootstrap, forecast, restart, and shutdown\n'
}

demo() {
  local scenario=${1:-all}
  if [[ $# -gt 0 ]]; then
    shift
  fi
  if ! python3 -c 'import sys; raise SystemExit(sys.version_info < (3, 10))'; then
    printf 'Release demonstrations require host Python 3.10 or newer.\n' >&2
    return 2
  fi
  python3 "$ROOT_DIR/scripts/release_demo.py" "$scenario" "$@"
}

if [[ $# -eq 0 ]]; then
  usage >&2
  exit 2
fi
if [[ $# -gt 1 && "${1:-}" != "logs" && "${1:-}" != "demo" ]]; then
  usage >&2
  exit 2
fi

command=${1:-}
shift || true
case "$command" in
  quickstart) quickstart "$@" ;;
  status) status "$@" ;;
  logs) logs "$@" ;;
  down) down "$@" ;;
  reset) reset "$@" ;;
  smoke) smoke "$@" ;;
  demo) demo "$@" ;;
  *) usage >&2; exit 2 ;;
esac
