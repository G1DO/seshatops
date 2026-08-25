#!/usr/bin/env python3
"""Bounded packaged-runtime release demonstrations for the local stack."""

from __future__ import annotations

import argparse
import hashlib
import http.cookiejar
import json
import math
import os
import re
import shlex
import stat
import subprocess
import sys
import tempfile
import threading
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable, Mapping
from urllib.error import HTTPError
from urllib.parse import parse_qs, urlencode, urljoin, urlparse
from urllib.request import (
    HTTPRedirectHandler,
    HTTPCookieProcessor,
    ProxyHandler,
    Request,
    build_opener,
)


ROOT = Path(__file__).resolve().parents[1]
COMPOSE_FILE = ROOT / "compose.yaml"
PROJECT_NAME = "seshatops-local"
BASE_URL = "http://web.seshatops.localhost:5173"
TENANT_ID = "11111111-1111-4111-8111-111111111111"
NEGATIVE_TENANT_ID = "22222222-2222-4222-8222-222222222222"
ITEM_ID = "item-flour-001"
BATCH_ID = "batch-bread-001"
ORDER_ID = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3020"
FINAL_EVENT_ID = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3015"
FIXTURE_VERSION = "northstar-m3-lineage-v1"
POISON_FIXTURE_VERSION = "northstar-m5-poison-v1"
FORECAST_INCOMPLETE_FIXTURE_VERSION = "northstar-m5-forecast-incomplete-v1"
FORECAST_HISTORY_FIXTURE_VERSION = "northstar-m4-stockout-v1"
HARNESS_VERSION = "m5-local-release-v1"
SCHEMA_VERSION = "seshatops.release-demo/v1"
DEMO_CONFIRM_ENV = "SESHATOPS_DEMO_CONFIRM"
FIXTURE_CONFIRM_ENV = "SESHATOPS_DEMO_FIXTURE_CONFIRM"
DEMO_CONFIRMATION = "I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO"
FORECAST_CONFIRMATION = "I_UNDERSTAND_FROZEN_M4_FORECAST_WRITE"
MAX_FAILURE_TEXT = 1200
MAX_DIAGNOSTIC_BYTES = 64 * 1024
MAX_SOURCE_BYTES = 64 * 1024 * 1024
MAX_RESULT_BYTES = 256 * 1024


SCENARIO_ORDER = (
    "normal-flow",
    "duplicate-delivery",
    "poison-isolation",
    "broker-recovery",
    "deterministic-rebuild",
    "tenant-isolation",
    "forecast-source-states",
    "python-degradation",
)


@dataclass(frozen=True)
class ScenarioSpec:
    preconditions: tuple[str, ...]
    expected_outcomes: tuple[str, ...]
    limitations: tuple[str, ...]


COMMON_LIMITATIONS = (
    "Evidence is from one disposable local Compose environment, not production.",
    "Observed durations are diagnostic measurements, not availability or recovery objectives.",
    "Redpanda transport remains at-least-once; duplicate delivery is expected behavior.",
)


SCENARIO_SPECS = {
    "normal-flow": ScenarioSpec(
        preconditions=(
            "Fresh packaged stack with empty disposable PostgreSQL and Redpanda volumes.",
            "Real local OIDC session and tenant-scoped reader authorization.",
            "SSE subscription established before the ERP fixture transaction.",
        ),
        expected_outcomes=(
            "The real ERP source transaction creates durable outbox intent.",
            "The runtime relays through Redpanda and applies inbox, inventory, and lineage state.",
            "REST and SSE expose the same committed inventory identity and checksum.",
            "Required relay, consumer, backlog, and readiness telemetry is present.",
        ),
        limitations=COMMON_LIMITATIONS,
    ),
    "duplicate-delivery": ScenarioSpec(
        preconditions=(
            "Fresh stack with the complete deterministic Northstar fixture projected.",
            "Exact final inventory event bytes remain retained in the source outbox.",
        ),
        expected_outcomes=(
            "An exact broker redelivery records duplicate_noop.",
            "Inventory and lineage checksums and effect-row counts remain unchanged.",
        ),
        limitations=COMMON_LIMITATIONS,
    ),
    "poison-isolation": ScenarioSpec(
        preconditions=(
            "Fresh stack with no source fixture effects.",
            "Fault injection is restricted to the fixed local Redpanda topic and key.",
        ),
        expected_outcomes=(
            "The incompatible event is tenant-attributed and quarantined.",
            "Unrelated valid source work continues through REST and the core runtime stays ready.",
        ),
        limitations=COMMON_LIMITATIONS,
    ),
    "broker-recovery": ScenarioSpec(
        preconditions=(
            "Fresh healthy stack and authenticated release-observer session.",
            "The fault interrupts only the declared local Redpanda service; recovery may restart the packaged runtime.",
        ),
        expected_outcomes=(
            "Broker interruption produces visible runtime degradation.",
            "ERP acceptance retains a durable outbox backlog while the broker is unavailable.",
            "Broker and runtime restart restore readiness and eventually drain and project the backlog.",
        ),
        limitations=COMMON_LIMITATIONS
        + (
            "The bounded local recovery procedure restarts the Go process after Redpanda; OIDC sessions are process-local and must be re-established.",
        ),
    ),
    "deterministic-rebuild": ScenarioSpec(
        preconditions=(
            "Fresh stack with complete retained source, inbox, inventory, and lineage history.",
            "Real OIDC session has the explicit platform-operator matrix row.",
        ),
        expected_outcomes=(
            "The public tenant rebuild completes only after its authorization audit insert.",
            "Before, rebuild-result, and after inventory and lineage checksums are equal.",
        ),
        limitations=COMMON_LIMITATIONS,
    ),
    "tenant-isolation": ScenarioSpec(
        preconditions=(
            "Fresh complete allowed-tenant fixture.",
            "The authenticated principal has no assignment for the negative tenant.",
        ),
        expected_outcomes=(
            "Cross-tenant read and privileged mutation both return minimal 403 bodies.",
            "No allowed-tenant payload leaks and no source or projection effect is created.",
        ),
        limitations=COMMON_LIMITATIONS,
    ),
    "forecast-source-states": ScenarioSpec(
        preconditions=(
            "Fresh stack starts with no live forecast-source history.",
            "Stale and incomplete states are introduced only at source, broker, or outbox boundaries.",
        ),
        expected_outcomes=(
            "Empty sparse history is reported as insufficient with no usable rows.",
            "Durable unapplied outbox history is reported as stale with no usable rows.",
            "Malformed retained source history is reported as incomplete with no usable rows.",
        ),
        limitations=COMMON_LIMITATIONS
        + (
            "The frozen M4 evaluation fixture is intentionally separate from live Event Spine history.",
            "The bounded local stale-source recovery restarts the Go process after Redpanda; OIDC sessions are process-local and must be re-established.",
        ),
    ),
    "python-degradation": ScenarioSpec(
        preconditions=(
            "Fresh stack has zero persisted forecast predictions.",
            "Unavailable and timeout faults run in a separate packaged command process.",
        ),
        expected_outcomes=(
            "Unavailable and timed-out Python invocations exit non-zero with typed telemetry.",
            "Neither failure writes any prediction row.",
            "Core source, broker, projection, REST, and readiness behavior continues.",
        ),
        limitations=COMMON_LIMITATIONS
        + (
            "Python failure evidence covers the local one-shot command boundary, not a production scheduler.",
        ),
    ),
}


class DemoError(RuntimeError):
    """Base class for actionable harness failures."""


class GuardError(DemoError):
    pass


class CommandFailure(DemoError):
    pass


class CommandTimeout(DemoError):
    pass


class InvariantFailure(DemoError):
    pass


class ScenarioTimeout(DemoError):
    pass


@dataclass
class CommandResult:
    args: list[str]
    returncode: int
    stdout: str
    stderr: str
    duration_ms: int


class CommandRunner:
    def __init__(self, run_impl: Callable[..., subprocess.CompletedProcess[str]] | None = None):
        self._run_impl = run_impl or subprocess.run

    def run(
        self,
        args: list[str],
        *,
        timeout: float,
        input_text: str | None = None,
        env: dict[str, str] | None = None,
    ) -> CommandResult:
        started = time.monotonic()
        command_env = os.environ.copy()
        if env:
            command_env.update(env)
        try:
            completed = self._run_impl(
                args,
                cwd=ROOT,
                check=False,
                capture_output=True,
                text=True,
                input=input_text,
                env=command_env,
                timeout=timeout,
            )
        except subprocess.TimeoutExpired as error:
            raise CommandTimeout(f"command timed out after {timeout:.1f}s: {safe_command(args)}") from error
        return CommandResult(
            args=list(args),
            returncode=completed.returncode,
            stdout=completed.stdout or "",
            stderr=completed.stderr or "",
            duration_ms=elapsed_ms(started),
        )


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def elapsed_ms(started: float) -> int:
    return max(0, round((time.monotonic() - started) * 1000))


def bounded_text(value: str, limit: int = MAX_FAILURE_TEXT) -> str:
    value = " ".join(value.strip().split())
    if len(value) <= limit:
        return value
    return value[: limit - 3] + "..."


def safe_command(args: list[str]) -> str:
    redacted = []
    context_command = args[:3] == ["docker", "context", "inspect"]
    for index, arg in enumerate(args):
        if DEMO_CONFIRMATION in arg or FORECAST_CONFIRMATION in arg:
            name = arg.split("=", 1)[0] if "=" in arg else "confirmation"
            redacted.append(name + "=<confirmation>")
        elif context_command and index == 3:
            redacted.append("<selected-local-context>")
        elif arg.startswith("unix://"):
            redacted.append("<local-unix-endpoint>")
        elif arg == str(ROOT):
            redacted.append("<repository-root>")
        elif Path(arg).name.startswith("seshatops-release-demo-") and Path(arg).suffix == ".yaml":
            redacted.append("<validated-compose-snapshot>")
        else:
            redacted.append(arg)
    return bounded_text(shlex.join(redacted))


def redact_command_output(value: str, args: list[str]) -> str:
    replacements = {
        DEMO_CONFIRMATION: "<confirmation>",
        FORECAST_CONFIRMATION: "<confirmation>",
        str(ROOT): "<repository-root>",
    }
    if args[:3] == ["docker", "context", "inspect"] and len(args) > 3:
        replacements[args[3]] = "<selected-local-context>"
    for arg in args:
        if arg.startswith("unix://"):
            replacements[arg] = "<local-unix-endpoint>"
        elif Path(arg).name.startswith("seshatops-release-demo-") and Path(arg).suffix == ".yaml":
            replacements[arg] = "<validated-compose-snapshot>"
    for raw, replacement in replacements.items():
        if raw:
            value = value.replace(raw, replacement)
    return value


def parse_json_output(raw: str) -> dict[str, Any]:
    for line in reversed(raw.splitlines()):
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict):
            return value
    raise InvariantFailure("command did not emit a JSON object")


def stable_identity(values: dict[str, Any]) -> dict[str, Any]:
    try:
        canonical = json.dumps(
            values,
            sort_keys=True,
            separators=(",", ":"),
            ensure_ascii=True,
            allow_nan=False,
        )
    except (TypeError, ValueError) as error:
        raise InvariantFailure("deterministic identity contains a non-JSON value") from error
    return {
        "values": values,
        "sha256": hashlib.sha256(canonical.encode()).hexdigest(),
    }


def pinned_compose_command(endpoint: str, compose_file: Path, *args: str) -> list[str]:
    return [
        "docker",
        "--host",
        endpoint,
        "compose",
        "--project-name",
        PROJECT_NAME,
        "--project-directory",
        str(ROOT),
        "--file",
        str(compose_file),
        *args,
    ]


def release_metadata(runner: CommandRunner) -> dict[str, Any]:
    commit = runner.run(["git", "rev-parse", "HEAD"], timeout=10)
    version = runner.run(["git", "describe", "--always", "--dirty"], timeout=10)
    dirty = runner.run(["git", "status", "--porcelain=v1"], timeout=10)
    if commit.returncode != 0 or version.returncode != 0 or dirty.returncode != 0:
        raise DemoError("release metadata could not be read from Git")
    return {
        "version": version.stdout.strip(),
        "commit": commit.stdout.strip(),
        "worktree_dirty": bool(dirty.stdout.strip()),
        "source_sha256": source_digest(runner),
        "harness_version": HARNESS_VERSION,
    }


def source_digest(runner: CommandRunner) -> str:
    listed = runner.run(
        ["git", "ls-files", "-co", "--exclude-standard", "-z"],
        timeout=30,
    )
    if listed.returncode != 0:
        raise DemoError("release source files could not be enumerated from Git")
    paths = sorted(set(path for path in listed.stdout.split("\0") if path))
    digest = hashlib.sha256()
    total_bytes = 0
    for relative in paths:
        relative_path = Path(relative)
        if relative_path.is_absolute() or ".." in relative_path.parts:
            raise DemoError("release source enumeration left the repository")
        path = ROOT / relative_path
        try:
            metadata = os.lstat(path)
        except FileNotFoundError:
            record = {"path": relative, "type": "missing"}
            encoded = json.dumps(record, sort_keys=True, separators=(",", ":")).encode()
            digest.update(len(encoded).to_bytes(8, "big"))
            digest.update(encoded)
            continue
        record: dict[str, Any] = {
            "mode": f"{stat.S_IMODE(metadata.st_mode):o}",
            "path": relative,
        }
        if stat.S_ISLNK(metadata.st_mode):
            record.update({"target": os.readlink(path), "type": "symlink"})
        elif stat.S_ISDIR(metadata.st_mode):
            record["type"] = "directory"
        elif stat.S_ISREG(metadata.st_mode):
            file_digest = hashlib.sha256()
            file_size = 0
            with path.open("rb") as handle:
                while chunk := handle.read(1024 * 1024):
                    file_size += len(chunk)
                    total_bytes += len(chunk)
                    if total_bytes > MAX_SOURCE_BYTES:
                        raise DemoError("release source fingerprint exceeds the bounded 64 MiB limit")
                    file_digest.update(chunk)
            record.update(
                {
                    "content_sha256": file_digest.hexdigest(),
                    "size": file_size,
                    "type": "file",
                }
            )
        else:
            raise DemoError(f"release source contains unsupported file type: {relative}")
        encoded = json.dumps(record, sort_keys=True, separators=(",", ":")).encode()
        digest.update(len(encoded).to_bytes(8, "big"))
        digest.update(encoded)
    return digest.hexdigest()


def require_release_unchanged(runner: CommandRunner, expected: dict[str, Any]) -> dict[str, Any]:
    observed = release_metadata(runner)
    if observed != expected:
        raise InvariantFailure("release checkout changed while evidence was being collected")
    return observed


class ResultBuilder:
    def __init__(
        self,
        scenario: str,
        release: dict[str, Any],
        *,
        break_expectation: str | None = None,
    ):
        spec = SCENARIO_SPECS[scenario]
        self.scenario = scenario
        self.release = release
        self.started_at = utc_now()
        self.started_monotonic = time.monotonic()
        self.actions: list[dict[str, Any]] = []
        self.expectations: list[dict[str, Any]] = []
        self.observations: dict[str, Any] = {
            "http_statuses": {},
            "counts": {},
            "durations_ms": {},
            "checksums": {},
            "telemetry": {},
            "fixture_versions": {},
        }
        self.preconditions = list(spec.preconditions)
        self.expected_outcomes = list(spec.expected_outcomes)
        self.limitations = list(spec.limitations)
        self.deterministic_values: dict[str, Any] = {}
        self.break_expectation = break_expectation
        self.break_seen = False

    def action(
        self,
        name: str,
        command: str,
        started_at: str,
        duration_ms: int,
        status: str,
        exit_code: int | None = None,
    ) -> None:
        record: dict[str, Any] = {
            "name": name,
            "command": command,
            "started_at": started_at,
            "duration_ms": duration_ms,
            "status": status,
        }
        if exit_code is not None:
            record["exit_code"] = exit_code
        self.actions.append(record)

    def expect(self, name: str, condition: bool, expected: Any, observed: Any) -> None:
        if self.break_expectation == name:
            self.break_seen = True
            condition = False
            observed = f"forced verification failure; actual={observed!r}"
        record = {
            "name": name,
            "expected": expected,
            "observed": observed,
            "passed": bool(condition),
        }
        self.expectations.append(record)
        if not condition:
            raise InvariantFailure(
                f"{name}: expected {bounded_text(str(expected))}; observed {bounded_text(str(observed))}"
            )

    def finish(
        self,
        *,
        status: str,
        cleanup: dict[str, Any],
        failure: dict[str, str] | None,
        diagnostics: list[str],
    ) -> dict[str, Any]:
        if self.break_expectation and not self.break_seen and failure is None:
            status = "failed"
            failure = {
                "category": "configuration",
                "message": f"unknown break expectation {self.break_expectation!r}",
            }
        result = {
            "schema_version": SCHEMA_VERSION,
            "kind": "scenario_result",
            "scenario": self.scenario,
            "release": self.release,
            "fixture_version": FIXTURE_VERSION,
            "fixture_versions": self.observations["fixture_versions"],
            "started_at": self.started_at,
            "finished_at": utc_now(),
            "duration_ms": elapsed_ms(self.started_monotonic),
            "preconditions": self.preconditions,
            "actions": self.actions,
            "expected_outcomes": self.expected_outcomes,
            "expectations": self.expectations,
            "observations": self.observations,
            "deterministic_identity": stable_identity(
                {
                    **self.deterministic_values,
                    "fixture_versions": self.observations["fixture_versions"],
                }
            ),
            "status": status,
            "failure": failure,
            "cleanup": cleanup,
            "diagnostics": diagnostics,
            "limitations": self.limitations,
        }
        validate_result_schema(result)
        return result


def validate_result_schema(result: dict[str, Any]) -> None:
    required = {
        "schema_version",
        "kind",
        "scenario",
        "release",
        "fixture_version",
        "fixture_versions",
        "started_at",
        "finished_at",
        "duration_ms",
        "preconditions",
        "actions",
        "expected_outcomes",
        "expectations",
        "observations",
        "deterministic_identity",
        "status",
        "failure",
        "cleanup",
        "diagnostics",
        "limitations",
    }
    missing = sorted(required - set(result))
    if missing:
        raise InvariantFailure(f"result schema missing fields: {', '.join(missing)}")
    if result["schema_version"] != SCHEMA_VERSION or result["kind"] != "scenario_result":
        raise InvariantFailure("result schema identity is invalid")
    if result["scenario"] not in SCENARIO_ORDER:
        raise InvariantFailure("result scenario is invalid")
    if result["status"] not in {"passed", "failed"}:
        raise InvariantFailure("result status is invalid")
    if result["status"] == "passed" and result["failure"] is not None:
        raise InvariantFailure("passed result contains a failure")
    if result["status"] == "failed" and not isinstance(result["failure"], dict):
        raise InvariantFailure("failed result has no failure object")
    if not isinstance(result["actions"], list) or not isinstance(result["expectations"], list):
        raise InvariantFailure("result actions or expectations are not arrays")
    if result["status"] == "passed" and (
        not result["actions"]
        or not result["expectations"]
        or any(expectation.get("passed") is not True for expectation in result["expectations"])
    ):
        raise InvariantFailure("passed result has no complete action and expectation evidence")
    if not isinstance(result["fixture_versions"], dict) or (
        result["status"] == "passed" and not result["fixture_versions"]
    ):
        raise InvariantFailure("result fixture versions are invalid")
    if not isinstance(result["cleanup"], dict) or result["cleanup"].get("status") not in {
        "passed",
        "failed",
        "not_run",
    }:
        raise InvariantFailure("result cleanup status is invalid")
    release = result["release"]
    if (
        not isinstance(release, dict)
        or not release.get("commit")
        or not release.get("version")
        or not re.fullmatch(r"[0-9a-f]{64}", release.get("source_sha256", ""))
    ):
        raise InvariantFailure("result release identity is incomplete")
    identity = result["deterministic_identity"]
    if not isinstance(identity, dict) or not re.fullmatch(r"[0-9a-f]{64}", identity.get("sha256", "")):
        raise InvariantFailure("result deterministic identity is invalid")
    try:
        encoded = json.dumps(result, sort_keys=True, allow_nan=False)
    except (TypeError, ValueError) as error:
        raise InvariantFailure("result contains a non-JSON value") from error
    if len(encoded.encode()) > MAX_RESULT_BYTES:
        raise InvariantFailure("result exceeds the bounded 256 KiB schema limit")
    for timestamp_field in ("started_at", "finished_at"):
        try:
            datetime.fromisoformat(result[timestamp_field].replace("Z", "+00:00"))
        except (TypeError, ValueError) as error:
            raise InvariantFailure(f"result {timestamp_field} is invalid") from error


def validate_compose_target(
    config: dict[str, Any],
    *,
    project_name: str,
    compose_file: Path,
    confirmation: str,
) -> None:
    if confirmation != DEMO_CONFIRMATION:
        raise GuardError(f"{DEMO_CONFIRM_ENV} must equal {DEMO_CONFIRMATION}")
    if project_name != PROJECT_NAME:
        raise GuardError("demonstration project name is not declared")
    if compose_file.resolve() != COMPOSE_FILE.resolve():
        raise GuardError("demonstration Compose file is not declared")
    services = config.get("services")
    expected_services = {"postgres", "redpanda", "redpanda-init", "oidc", "runtime", "web"}
    if not isinstance(services, dict) or set(services) != expected_services:
        raise GuardError("demonstration services do not match the declared local package")
    if config.get("name") != PROJECT_NAME:
        raise GuardError("rendered Compose project name is not declared")

    expected_service_keys = {
        "postgres": {
            "command",
            "entrypoint",
            "environment",
            "healthcheck",
            "image",
            "networks",
            "volumes",
        },
        "redpanda": {
            "cap_drop",
            "command",
            "entrypoint",
            "healthcheck",
            "image",
            "networks",
            "security_opt",
            "volumes",
        },
        "redpanda-init": {
            "cap_drop",
            "command",
            "depends_on",
            "entrypoint",
            "image",
            "networks",
            "restart",
            "security_opt",
        },
        "oidc": {
            "cap_drop",
            "command",
            "entrypoint",
            "environment",
            "healthcheck",
            "image",
            "networks",
            "ports",
            "read_only",
            "security_opt",
            "tmpfs",
            "volumes",
        },
        "runtime": {
            "build",
            "cap_drop",
            "command",
            "depends_on",
            "entrypoint",
            "environment",
            "expose",
            "healthcheck",
            "networks",
            "read_only",
            "security_opt",
            "tmpfs",
        },
        "web": {
            "build",
            "cap_drop",
            "command",
            "depends_on",
            "entrypoint",
            "environment",
            "healthcheck",
            "networks",
            "ports",
            "read_only",
            "security_opt",
            "tmpfs",
        },
    }
    for service_name, expected_keys in expected_service_keys.items():
        if set(services[service_name]) != expected_keys:
            raise GuardError(
                f"{service_name} service fields differ from the fixed local package"
            )

    expected_images = {
        "postgres": "postgres@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b",
        "redpanda": "docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07",
        "redpanda-init": "docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07",
        "oidc": "ghcr.io/navikt/mock-oauth2-server@sha256:79f51f412caddb1e2120a5ae10d1f203e134f6e8328f1bc63c444acba33c9086",
    }
    for service_name, expected_image in expected_images.items():
        service = services[service_name]
        if service.get("image") != expected_image or service.get("build") is not None:
            raise GuardError(f"{service_name} image is not the pinned local package image")

    expected_builds = {
        "runtime": {"context": str(ROOT), "dockerfile": "docker/go.Dockerfile"},
        "web": {"context": str(ROOT), "dockerfile": "docker/web.Dockerfile"},
    }
    for service_name, expected_build in expected_builds.items():
        service = services[service_name]
        if service.get("build") != expected_build or service.get("image") is not None:
            raise GuardError(f"{service_name} build is not the declared packaged source")

    expected_commands = {
        "postgres": None,
        "redpanda": [
            "redpanda",
            "start",
            "--mode",
            "dev-container",
            "--overprovisioned",
            "--smp",
            "1",
            "--reserve-memory",
            "0M",
            "--check=false",
            "--kafka-addr",
            "internal://0.0.0.0:9092",
            "--advertise-kafka-addr",
            "internal://redpanda:9092",
            "--rpc-addr",
            "0.0.0.0:33145",
            "--advertise-rpc-addr",
            "redpanda:33145",
        ],
        "redpanda-init": [
            "-ec",
            "rpk topic create seshatops.m1.events --brokers redpanda:9092 --partitions 1 --replicas 1 || "
            "rpk topic describe seshatops.m1.events --brokers redpanda:9092",
        ],
        "oidc": None,
        "runtime": None,
        "web": [
            "npm",
            "run",
            "dev",
            "--",
            "--host",
            "0.0.0.0",
            "--configLoader",
            "runner",
        ],
    }
    expected_entrypoints = {
        "postgres": None,
        "redpanda": None,
        "redpanda-init": ["/bin/sh"],
        "oidc": None,
        "runtime": None,
        "web": None,
    }
    for service_name in expected_services:
        service = services[service_name]
        if service.get("command") != expected_commands[service_name]:
            raise GuardError(f"{service_name} command does not match the packaged local service")
        if service.get("entrypoint") != expected_entrypoints[service_name]:
            raise GuardError(f"{service_name} entrypoint does not match the packaged local service")

    expected_hardening = {
        "postgres": (None, None, None),
        "redpanda": (["ALL"], ["no-new-privileges:true"], None),
        "redpanda-init": (["ALL"], ["no-new-privileges:true"], None),
        "oidc": (["ALL"], ["no-new-privileges:true"], True),
        "runtime": (["ALL"], ["no-new-privileges:true"], True),
        "web": (["ALL"], ["no-new-privileges:true"], True),
    }
    for service_name, (cap_drop, security_opt, read_only) in expected_hardening.items():
        service = services[service_name]
        if (
            service.get("cap_drop") != cap_drop
            or service.get("security_opt") != security_opt
            or service.get("read_only") is not read_only
        ):
            raise GuardError(f"{service_name} security controls differ from the local package")
    if services["redpanda-init"].get("restart") != "no":
        raise GuardError("redpanda-init restart policy differs from the local package")
    for service_name in ("redpanda", "redpanda-init"):
        if services[service_name].get("environment") not in (None, {}):
            raise GuardError(f"{service_name} may not add undeclared environment controls")

    expected_lifecycle = {
        "postgres": {
            "depends_on": None,
            "expose": None,
            "healthcheck": {
                "interval": "2s",
                "retries": 30,
                "start_period": "5s",
                "test": [
                    "CMD-SHELL",
                    "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}",
                ],
                "timeout": "5s",
            },
            "restart": None,
            "tmpfs": None,
        },
        "redpanda": {
            "depends_on": None,
            "expose": None,
            "healthcheck": {
                "interval": "2s",
                "retries": 30,
                "start_period": "10s",
                "test": [
                    "CMD-SHELL",
                    "rpk cluster health --api-urls redpanda:9644 | grep -q Healthy",
                ],
                "timeout": "5s",
            },
            "restart": None,
            "tmpfs": None,
        },
        "redpanda-init": {
            "depends_on": {
                "redpanda": {"condition": "service_healthy", "required": True}
            },
            "expose": None,
            "healthcheck": None,
            "restart": "no",
            "tmpfs": None,
        },
        "oidc": {
            "depends_on": None,
            "expose": None,
            "healthcheck": {
                "interval": "2s",
                "retries": 30,
                "start_period": "5s",
                "test": [
                    "CMD",
                    "wget",
                    "--no-verbose",
                    "--tries=1",
                    "--spider",
                    "http://127.0.0.1:9090/isalive",
                ],
                "timeout": "5s",
            },
            "restart": None,
            "tmpfs": ["/tmp"],
        },
        "runtime": {
            "depends_on": {
                "oidc": {"condition": "service_healthy", "required": True},
                "postgres": {"condition": "service_healthy", "required": True},
                "redpanda-init": {
                    "condition": "service_completed_successfully",
                    "required": True,
                },
            },
            "expose": ["8080"],
            "healthcheck": {
                "interval": "2s",
                "retries": 60,
                "start_period": "10s",
                "test": [
                    "CMD",
                    "python3",
                    "-c",
                    "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8080/readyz', timeout=2)",
                ],
                "timeout": "5s",
            },
            "restart": None,
            "tmpfs": ["/tmp"],
        },
        "web": {
            "depends_on": {
                "runtime": {"condition": "service_healthy", "required": True}
            },
            "expose": None,
            "healthcheck": {
                "interval": "2s",
                "retries": 30,
                "start_period": "5s",
                "test": [
                    "CMD",
                    "node",
                    "-e",
                    "fetch('http://127.0.0.1:5173/').then((r) => { if (!r.ok) process.exit(1) }).catch(() => process.exit(1))",
                ],
                "timeout": "5s",
            },
            "restart": None,
            "tmpfs": ["/tmp"],
        },
    }
    for service_name, lifecycle in expected_lifecycle.items():
        service = services[service_name]
        if any(service.get(field) != value for field, value in lifecycle.items()):
            raise GuardError(
                f"{service_name} lifecycle differs from the fixed local package"
            )

    volumes = config.get("volumes")
    expected_volume_names = {
        "postgres-data": f"{PROJECT_NAME}_postgres-data",
        "redpanda-data": f"{PROJECT_NAME}_redpanda-data",
    }
    if not isinstance(volumes, dict) or set(volumes) != set(expected_volume_names):
        raise GuardError("demonstration volumes do not match the disposable package")
    for volume_key, volume_name in expected_volume_names.items():
        declaration = volumes[volume_key]
        if not isinstance(declaration, dict) or declaration != {"name": volume_name}:
            raise GuardError(f"volume {volume_key} is not a project-scoped disposable volume")

    networks = config.get("networks")
    if not isinstance(networks, dict) or set(networks) != {"local"}:
        raise GuardError("demonstration network does not match the local package")
    local_network = networks["local"]
    expected_network = {"name": f"{PROJECT_NAME}_local"}
    if local_network not in (expected_network, {**expected_network, "ipam": {}}):
        raise GuardError("demonstration network is not project-scoped and local")

    expected_service_networks: dict[str, dict[str, Any]] = {
        "postgres": {"local": None},
        "redpanda": {"local": None},
        "redpanda-init": {"local": None},
        "runtime": {"local": None},
        "oidc": {"local": {"aliases": ["oidc.seshatops.localhost"]}},
        "web": {"local": {"aliases": ["web.seshatops.localhost"]}},
    }
    forbidden_network_controls = (
        "extra_hosts",
        "network_mode",
        "links",
        "external_links",
        "dns",
        "dns_search",
    )
    for service_name, expected_networks in expected_service_networks.items():
        service = services[service_name]
        if service.get("networks") != expected_networks:
            raise GuardError(f"{service_name} is not attached only to the project-local network")
        if any(service.get(control) not in (None, [], {}) for control in forbidden_network_controls):
            raise GuardError(f"{service_name} may not override local service resolution")
        if service.get("volumes_from") not in (None, []):
            raise GuardError(f"{service_name} may not inherit undeclared volumes")

    expected_data_mounts = {
        "postgres": ("postgres-data", "/var/lib/postgresql/data"),
        "redpanda": ("redpanda-data", "/var/lib/redpanda/data"),
    }
    for service_name, (source, target) in expected_data_mounts.items():
        mounts = services[service_name].get("volumes")
        if not isinstance(mounts, list) or len(mounts) != 1:
            raise GuardError(f"{service_name} data mount is not declared exactly once")
        mount = mounts[0]
        if (
            not isinstance(mount, dict)
            or mount.get("type") != "volume"
            or mount.get("source") != source
            or mount.get("target") != target
            or mount.get("read_only") is True
        ):
            raise GuardError(f"{service_name} data mount is not the disposable project volume")

    oidc_mounts = services["oidc"].get("volumes")
    expected_oidc_source = (ROOT / "docker" / "oidc" / "config.json").resolve()
    if not isinstance(oidc_mounts, list) or len(oidc_mounts) != 1:
        raise GuardError("OIDC configuration mount is not declared exactly once")
    oidc_mount = oidc_mounts[0]
    if (
        not isinstance(oidc_mount, dict)
        or oidc_mount.get("type") != "bind"
        or Path(str(oidc_mount.get("source", ""))).resolve() != expected_oidc_source
        or oidc_mount.get("target") != "/app/config.json"
        or oidc_mount.get("read_only") is not True
    ):
        raise GuardError("OIDC configuration mount is not the fixed read-only local fixture")
    for service_name in ("redpanda-init", "runtime", "web"):
        if services[service_name].get("volumes") not in (None, []):
            raise GuardError(f"{service_name} may not add demonstration mounts")

    runtime = services["runtime"]
    environment = runtime.get("environment", {})
    if environment.get("SESHATOPS_LOCAL_STACK") != "true":
        raise GuardError("runtime is not explicitly declared as the local stack")
    database_url = environment.get("SESHATOPS_DATABASE_URL", "")
    parsed = urlparse(database_url)
    if parsed.scheme not in {"postgres", "postgresql"}:
        raise GuardError("runtime database is not PostgreSQL")
    if (
        parsed.hostname != "postgres"
        or parsed.username != "seshatops"
        or parsed.path.lstrip("/") != "seshatops_northstar_disposable"
    ):
        raise GuardError("runtime database is not the declared disposable local target")
    try:
        database_port = parsed.port
    except ValueError as error:
        raise GuardError("runtime database port is invalid") from error
    if database_port != 5432:
        raise GuardError("runtime database port is not the declared disposable local target")
    database_query = parse_qs(parsed.query, keep_blank_values=True)
    if database_query != {"sslmode": ["disable"]}:
        raise GuardError("runtime database query may not override the declared local target")
    if environment.get("SESHATOPS_BROKER_SEEDS") != "redpanda:9092":
        raise GuardError("runtime broker is not the declared local Redpanda target")
    expected_runtime_environment = {
        "SESHATOPS_AUTH_ASSIGNMENTS": (
            "northstar-demo-operator|11111111-1111-4111-8111-111111111111|ROLE-OPS-READER, "
            "northstar-demo-operator|11111111-1111-4111-8111-111111111111|ROLE-PLATFORM-OPERATOR, "
            "northstar-demo-operator|SCOPE-RUNTIME|ROLE-RELEASE-OBSERVER"
        ),
        "SESHATOPS_BROKER_SEEDS": "redpanda:9092",
        "SESHATOPS_COOKIE_NAME": "seshatops_session",
        "SESHATOPS_COOKIE_SECURE": "false",
        "SESHATOPS_DATABASE_URL": (
            "postgres://seshatops:seshatops-local-only@postgres:5432/"
            "seshatops_northstar_disposable?sslmode=disable"
        ),
        "SESHATOPS_FORECAST_PYTHON": "python3",
        "SESHATOPS_LISTEN_ADDR": ":8080",
        "SESHATOPS_LOCAL_STACK": "true",
        "SESHATOPS_OIDC_AUDIENCE": "seshatops-local",
        "SESHATOPS_OIDC_CLIENT_ID": "seshatops-local",
        "SESHATOPS_OIDC_ISSUER": "http://oidc.seshatops.localhost:9090/default",
        "SESHATOPS_OIDC_REDIRECT_URL": "http://web.seshatops.localhost:5173/auth/callback",
    }
    if environment != expected_runtime_environment:
        raise GuardError("runtime environment does not match the declared local package")

    expected_postgres_environment = {
        "POSTGRES_DB": "seshatops_northstar_disposable",
        "POSTGRES_PASSWORD": "seshatops-local-only",
        "POSTGRES_USER": "seshatops",
    }
    if services["postgres"].get("environment") != expected_postgres_environment:
        raise GuardError("PostgreSQL environment does not match the disposable package")

    expected_oidc_environment = {
        "JSON_CONFIG_PATH": "/app/config.json",
        "LOG_LEVEL": "warn",
        "SERVER_PORT": "9090",
    }
    if services["oidc"].get("environment") != expected_oidc_environment:
        raise GuardError("OIDC environment does not match the declared local package")

    expected_web_environment = {
        "VITE_API_PROXY_TARGET": "http://runtime:8080",
        "VITE_CACHE_DIR": "/tmp/seshatops-vite",
    }
    if services["web"].get("environment") != expected_web_environment:
        raise GuardError("web proxy target is not the declared local runtime")

    expected_loopback_ports = {"oidc": 9090, "web": 5173}
    for service_name, expected_port in expected_loopback_ports.items():
        ports = services[service_name].get("ports")
        if not isinstance(ports, list) or len(ports) != 1:
            raise GuardError(f"{service_name} must publish exactly one local loopback port")
        port = ports[0]
        if (
            not isinstance(port, dict)
            or port.get("host_ip") != "127.0.0.1"
            or port.get("target") != expected_port
            or str(port.get("published")) != str(expected_port)
            or port.get("protocol") != "tcp"
            or port.get("mode") != "ingress"
        ):
            raise GuardError(f"{service_name} port is not the declared local loopback endpoint")
    for service_name in ("postgres", "redpanda", "runtime"):
        if services[service_name].get("ports"):
            raise GuardError(f"{service_name} must not expose host ports for demonstrations")


def validate_docker_environment(environment: Mapping[str, str]) -> None:
    overrides = [
        name
        for name in ("DOCKER_HOST", "DOCKER_CONTEXT")
        if environment.get(name, "").strip()
    ]
    if overrides:
        raise GuardError(
            "Docker target overrides must be unset for demonstrations: "
            + ", ".join(overrides)
        )


def validate_local_docker_endpoint(endpoint: str) -> None:
    parsed = urlparse(endpoint.strip())
    if (
        parsed.scheme != "unix"
        or parsed.netloc
        or not parsed.path.startswith("/")
        or parsed.params
        or parsed.query
        or parsed.fragment
    ):
        raise GuardError("demonstrations require a local Unix Docker endpoint")


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class SessionClient:
    def __init__(self, base_url: str = BASE_URL):
        self.base_url = base_url
        self.jar = http.cookiejar.CookieJar()
        self.opener = build_opener(
            ProxyHandler({}),
            HTTPCookieProcessor(self.jar),
            NoRedirect(),
        )

    def request(
        self,
        path_or_url: str,
        *,
        method: str = "GET",
        json_body: dict[str, Any] | None = None,
        accept: str = "application/json, text/html",
        timeout: float = 10,
    ) -> tuple[int, dict[str, str], bytes]:
        url = path_or_url if path_or_url.startswith("http") else self.base_url + path_or_url
        data = None
        headers = {"Accept": accept}
        if json_body is not None:
            data = json.dumps(json_body, separators=(",", ":")).encode()
            headers["Content-Type"] = "application/json"
        request = Request(url, data=data, headers=headers, method=method)
        try:
            with self.opener.open(request, timeout=timeout) as response:
                return response.status, dict(response.headers), response.read()
        except HTTPError as error:
            return error.code, dict(error.headers), error.read()

    def login(self) -> dict[str, Any]:
        status, headers, _ = self.request("/auth/login")
        authorize_url = header(headers, "Location")
        if status != 302 or not authorize_url:
            raise InvariantFailure(f"OIDC login redirect status={status}")
        query = parse_qs(urlparse(authorize_url).query)
        if query.get("code_challenge_method") != ["S256"]:
            raise InvariantFailure("OIDC login did not use PKCE S256")
        if query.get("redirect_uri") != [self.base_url + "/auth/callback"]:
            raise InvariantFailure("OIDC callback is not same-origin")

        status, _, body = self.request(authorize_url)
        if status != 200:
            raise InvariantFailure(f"OIDC authorization page status={status}")
        form = re.search(rb"<form\b[^>]*>", body, re.IGNORECASE)
        if form is None:
            raise InvariantFailure("OIDC authorization page has no login form")
        action = re.search(rb'\saction=["\']([^"\']+)', form.group(0), re.IGNORECASE)
        login_url = urljoin(authorize_url, action.group(1).decode() if action else authorize_url)
        request = Request(
            login_url,
            data=urlencode({"username": "northstar-demo-operator"}).encode(),
            headers={"Accept": "text/html"},
            method="POST",
        )
        try:
            self.opener.open(request, timeout=10)
            raise InvariantFailure("OIDC form unexpectedly followed redirect")
        except HTTPError as error:
            status = error.code
            headers = dict(error.headers)
            error.read()
        callback_url = header(headers, "Location")
        if status != 302 or not callback_url:
            raise InvariantFailure(f"OIDC form status={status}")
        if urlparse(callback_url).netloc != urlparse(self.base_url).netloc:
            raise InvariantFailure("OIDC callback left the web origin")
        status, headers, _ = self.request(callback_url)
        if status != 302 or not header(headers, "Location"):
            raise InvariantFailure(f"OIDC callback status={status}")
        status, _, body = self.request("/auth/session")
        session = decode_json(body)
        if status != 200 or session.get("principal_id") != "northstar-demo-operator":
            raise InvariantFailure(f"OIDC session status={status}")
        return session


def header(headers: dict[str, str], name: str) -> str | None:
    return next((value for key, value in headers.items() if key.lower() == name.lower()), None)


def decode_json(raw: bytes | str) -> dict[str, Any]:
    try:
        value = json.loads(raw)
    except (json.JSONDecodeError, UnicodeDecodeError) as error:
        raise InvariantFailure("HTTP response was not valid JSON") from error
    if not isinstance(value, dict):
        raise InvariantFailure("HTTP response was not a JSON object")
    return value


class SSECapture:
    def __init__(self, client: SessionClient, path: str, expected_event: str):
        self.client = client
        self.path = path
        self.expected_event = expected_event
        self.ready = threading.Event()
        self.done = threading.Event()
        self.payload: dict[str, Any] | None = None
        self.error: BaseException | None = None
        self.response = None
        self.thread = threading.Thread(target=self._read, daemon=True)

    def start(self) -> None:
        self.thread.start()
        # Vite can buffer the upstream response headers until the first event.
        # The Go handler sends a heartbeat every 15 seconds, so allow that
        # bounded flush before accepting the source transaction. This proves
        # the subscriber was registered before the committed update occurs.
        if not self.ready.wait(20):
            self.close()
            raise ScenarioTimeout("SSE subscription was not established within 20.0s")
        if self.error:
            raise InvariantFailure(f"SSE subscription failed: {bounded_text(str(self.error))}")

    def _read(self) -> None:
        request = Request(
            self.client.base_url + self.path,
            headers={"Accept": "text/event-stream"},
            method="GET",
        )
        try:
            self.response = self.client.opener.open(request, timeout=60)
            if self.response.status != 200:
                raise InvariantFailure(f"SSE status={self.response.status}")
            event_name = ""
            for raw_line in self.response:
                line = raw_line.decode("utf-8").rstrip("\r\n")
                if line == ": heartbeat":
                    self.ready.set()
                elif line.startswith("event: "):
                    event_name = line[7:]
                elif line.startswith("data: ") and event_name == self.expected_event:
                    self.payload = decode_json(line[6:])
                    self.done.set()
                    return
        except BaseException as error:  # retained for the coordinating thread
            self.error = error
            self.ready.set()
            self.done.set()

    def wait(self, timeout: float) -> dict[str, Any]:
        if not self.done.wait(timeout):
            self.close()
            raise ScenarioTimeout(f"SSE event {self.expected_event!r} was not observed within {timeout:.1f}s")
        if self.error:
            raise InvariantFailure(f"SSE read failed: {bounded_text(str(self.error))}")
        if self.payload is None:
            raise InvariantFailure("SSE stream ended without the expected payload")
        self.close()
        return self.payload

    def close(self) -> None:
        if self.response is not None:
            self.response.close()


def wait_until(
    fetch: Callable[[], Any],
    predicate: Callable[[Any], bool],
    *,
    timeout: float,
    description: str,
    interval: float = 0.5,
) -> tuple[Any, int]:
    started = time.monotonic()
    attempts = 0
    last: Any = None
    while True:
        attempts += 1
        last = fetch()
        if predicate(last):
            return last, attempts
        if time.monotonic() - started >= timeout:
            raise ScenarioTimeout(f"{description} was not observed within {timeout:.1f}s; last={bounded_text(str(last))}")
        time.sleep(interval)


def metric_value(text: str, metric: str, labels: str = "") -> float:
    prefix = metric + ("{" + labels + "}" if labels else "") + " "
    for line in text.splitlines():
        if line.startswith(prefix):
            try:
                value = float(line[len(prefix) :].strip())
            except ValueError as error:
                raise InvariantFailure(f"telemetry {metric} is not numeric") from error
            if not math.isfinite(value):
                raise InvariantFailure(f"telemetry {metric} is not finite")
            return value
    raise InvariantFailure(f"required telemetry {metric}{'{' + labels + '}' if labels else ''} is missing")


DB_QUERIES = {
    "effect-counts": """
        SELECT json_build_object(
          'inventory_rows', (SELECT COUNT(*) FROM platform.inventory_projection WHERE tenant_id = '11111111-1111-4111-8111-111111111111'),
          'lineage_rows',
            (SELECT COUNT(*) FROM platform.lineage_suppliers WHERE tenant_id = '11111111-1111-4111-8111-111111111111') +
            (SELECT COUNT(*) FROM platform.lineage_ingredient_lots WHERE tenant_id = '11111111-1111-4111-8111-111111111111') +
            (SELECT COUNT(*) FROM platform.lineage_production_batches WHERE tenant_id = '11111111-1111-4111-8111-111111111111') +
            (SELECT COUNT(*) FROM platform.lineage_shipments WHERE tenant_id = '11111111-1111-4111-8111-111111111111')
        )
    """,
    "foreign-effects": """
        SELECT json_build_object(
          'source_rows', (SELECT COUNT(*) FROM erp.outbox WHERE tenant_id = '22222222-2222-4222-8222-222222222222'),
          'inbox_rows', (SELECT COUNT(*) FROM platform.inbox WHERE tenant_id = '22222222-2222-4222-8222-222222222222'),
          'inventory_rows', (SELECT COUNT(*) FROM platform.inventory_projection WHERE tenant_id = '22222222-2222-4222-8222-222222222222'),
          'lineage_rows',
            (SELECT COUNT(*) FROM platform.lineage_suppliers WHERE tenant_id = '22222222-2222-4222-8222-222222222222') +
            (SELECT COUNT(*) FROM platform.lineage_ingredient_lots WHERE tenant_id = '22222222-2222-4222-8222-222222222222') +
            (SELECT COUNT(*) FROM platform.lineage_production_batches WHERE tenant_id = '22222222-2222-4222-8222-222222222222') +
            (SELECT COUNT(*) FROM platform.lineage_shipments WHERE tenant_id = '22222222-2222-4222-8222-222222222222')
        )
    """,
    "foreign-deny-audits": """
        SELECT json_build_object(
          'rebuild_denials', (
            SELECT COUNT(*)
            FROM identity.authorization_decisions
            WHERE principal_id = 'northstar-demo-operator'
              AND tenant_id = '22222222-2222-4222-8222-222222222222'
              AND resource = 'RES-REBUILD'
              AND action = 'ACT-REBUILD'
              AND outcome = 'deny'
          )
        )
    """,
    "allowed-rebuild-audits": """
        SELECT json_build_object(
          'rebuild_allows', (
            SELECT COUNT(*)
            FROM identity.authorization_decisions
            WHERE principal_id = 'northstar-demo-operator'
              AND tenant_id = '11111111-1111-4111-8111-111111111111'
              AND resource = 'RES-REBUILD'
              AND action = 'ACT-REBUILD'
              AND outcome = 'allow'
          )
        )
    """,
    "prediction-count": """
        SELECT json_build_object(
          'prediction_rows', (SELECT COUNT(*) FROM platform.forecast_predictions)
        )
    """,
    "poison-effects": """
        SELECT json_build_object(
          'inbox_rows', (
            SELECT COUNT(*) FROM platform.inbox
            WHERE event_id = '318f5d78-6e64-4f5f-bd16-8e9f7c4a4011'
          ),
          'projection_rows',
            (SELECT COUNT(*) FROM platform.lineage_suppliers
             WHERE source_event_id = '318f5d78-6e64-4f5f-bd16-8e9f7c4a4011'
                OR supplier_id = 'mill-northstar-poison-001') +
            (SELECT COUNT(*) FROM platform.lineage_ingredient_lots
             WHERE source_event_id = '318f5d78-6e64-4f5f-bd16-8e9f7c4a4011') +
            (SELECT COUNT(*) FROM platform.lineage_production_batches
             WHERE source_event_id = '318f5d78-6e64-4f5f-bd16-8e9f7c4a4011') +
            (SELECT COUNT(*) FROM platform.lineage_shipments
             WHERE source_event_id = '318f5d78-6e64-4f5f-bd16-8e9f7c4a4011'),
          'failure_rows', (
            SELECT COUNT(*) FROM platform.processing_failures
            WHERE event_id = '318f5d78-6e64-4f5f-bd16-8e9f7c4a4011'
          )
        )
    """,
}


class DemoDriver:
    def __init__(self, runner: CommandRunner, builder: ResultBuilder, evidence_dir: Path):
        self.runner = runner
        self.builder = builder
        self.evidence_dir = evidence_dir
        self.guarded = False
        self.docker_endpoint: str | None = None
        self.compose_snapshot: Path | None = None
        self.owns_compose_snapshot = False

    def compose_command(self, *args: str) -> list[str]:
        if self.docker_endpoint is None or self.compose_snapshot is None:
            raise GuardError("Docker endpoint and Compose package have not been validated")
        return pinned_compose_command(self.docker_endpoint, self.compose_snapshot, *args)

    def verify_release_identity(self, expected: dict[str, Any], label: str) -> None:
        observed = release_metadata(self.runner)
        self.builder.expect(
            f"release_identity_{label}",
            observed == expected,
            expected,
            observed,
        )

    def snapshot_compose_package(self) -> None:
        try:
            source = COMPOSE_FILE.read_bytes()
            descriptor, raw_path = tempfile.mkstemp(prefix="seshatops-release-demo-", suffix=".yaml")
            with os.fdopen(descriptor, "wb") as handle:
                handle.write(source)
        except OSError as error:
            raise GuardError("could not snapshot the declared Compose package") from error
        self.compose_snapshot = Path(raw_path)
        self.owns_compose_snapshot = True

    def remove_compose_snapshot(self) -> str | None:
        if not self.owns_compose_snapshot or self.compose_snapshot is None:
            return None
        try:
            self.compose_snapshot.unlink(missing_ok=True)
        except OSError as error:
            detail = str(error).replace(
                str(self.compose_snapshot), "<validated-compose-snapshot>"
            )
            return bounded_text(f"could not remove Compose snapshot: {detail}")
        finally:
            self.compose_snapshot = None
            self.owns_compose_snapshot = False
        return None

    def command(
        self,
        name: str,
        args: list[str],
        *,
        timeout: float,
        expected_codes: set[int] | None = None,
        input_text: str | None = None,
    ) -> CommandResult:
        expected_codes = expected_codes or {0}
        started_at = utc_now()
        started = time.monotonic()
        try:
            result = self.runner.run(args, timeout=timeout, input_text=input_text)
        except CommandTimeout:
            self.builder.action(name, safe_command(args), started_at, elapsed_ms(started), "failed")
            raise
        status = "passed" if result.returncode in expected_codes else "failed"
        self.builder.action(
            name,
            safe_command(args),
            started_at,
            result.duration_ms,
            status,
            result.returncode,
        )
        if result.returncode not in expected_codes:
            detail = result.stderr or result.stdout or "no diagnostic output"
            raise CommandFailure(
                f"{name} exited {result.returncode}: {bounded_text(redact_command_output(detail, args))}"
            )
        return result

    def step(self, name: str, command: str, function: Callable[[], Any]) -> Any:
        started_at = utc_now()
        started = time.monotonic()
        try:
            value = function()
        except BaseException:
            self.builder.action(name, command, started_at, elapsed_ms(started), "failed")
            raise
        self.builder.action(name, command, started_at, elapsed_ms(started), "passed")
        return value

    def guard_target(self) -> None:
        validate_docker_environment(os.environ)
        context_result = self.command(
            "resolve selected Docker context",
            ["docker", "context", "show"],
            timeout=10,
        )
        context_name = context_result.stdout.strip()
        if not context_name or "\n" in context_name or "\r" in context_name:
            raise GuardError("selected Docker context is invalid")
        endpoint_result = self.command(
            "validate local Docker endpoint",
            [
                "docker",
                "context",
                "inspect",
                context_name,
                "--format",
                "{{json .Endpoints.docker.Host}}",
            ],
            timeout=10,
        )
        try:
            endpoint = json.loads(endpoint_result.stdout)
        except json.JSONDecodeError as error:
            raise GuardError("selected Docker context endpoint is invalid") from error
        if not isinstance(endpoint, str):
            raise GuardError("selected Docker context endpoint is invalid")
        validate_local_docker_endpoint(endpoint)
        self.docker_endpoint = endpoint
        self.snapshot_compose_package()
        result = self.command(
            "validate declared Compose target",
            self.compose_command("config", "--format", "json"),
            timeout=30,
        )
        try:
            config = json.loads(result.stdout)
        except json.JSONDecodeError as error:
            raise GuardError("Compose target did not render valid JSON") from error
        validate_compose_target(
            config,
            project_name=PROJECT_NAME,
            compose_file=COMPOSE_FILE,
            confirmation=os.environ.get(DEMO_CONFIRM_ENV, ""),
        )
        self.guarded = True
        self.builder.expect(
            "declared_local_target",
            True,
            "fixed seshatops-local Compose project and disposable dependencies",
            "guarded",
        )

    def fresh_start(self, expected_release: dict[str, Any]) -> None:
        self.guard_target()
        self.command(
            "reset disposable environment before scenario",
            self.compose_command("down", "--volumes", "--remove-orphans"),
            timeout=120,
        )
        self.verify_release_identity(expected_release, "before_packaged_build")
        self.command(
            "build and start packaged environment",
            self.compose_command("up", "--build", "--detach", "--wait", "--wait-timeout", "180"),
            timeout=600,
        )
        self.verify_release_identity(expected_release, "after_packaged_build")

    def fixture(self, action: str) -> dict[str, Any]:
        if action not in {"source", "inspect", "duplicate", "poison", "forecast-incomplete"}:
            raise GuardError(f"undeclared fixture action {action!r}")
        result = self.command(
            f"run guarded fixture action {action}",
            self.compose_command(
                "exec",
                "-T",
                "-e",
                f"{FIXTURE_CONFIRM_ENV}={DEMO_CONFIRMATION}",
                "runtime",
                "/app/seshatops",
                "demo-fixture",
                action,
            ),
            timeout=60,
        )
        summary = parse_json_output(result.stdout)
        self.builder.expect(
            f"fixture_{action}_status",
            summary.get("status") == "complete",
            "complete",
            summary.get("status"),
        )
        expected_fixture_version = {
            "source": FIXTURE_VERSION,
            "inspect": FIXTURE_VERSION,
            "duplicate": FIXTURE_VERSION,
            "poison": POISON_FIXTURE_VERSION,
            "forecast-incomplete": FORECAST_INCOMPLETE_FIXTURE_VERSION,
        }[action]
        self.builder.expect(
            f"fixture_{action}_version",
            summary.get("fixture_version") == expected_fixture_version,
            expected_fixture_version,
            summary.get("fixture_version"),
        )
        self.builder.observations["fixture_versions"][action] = expected_fixture_version
        return summary

    def checkpoint_summary(self) -> dict[str, Any]:
        return self.fixture("inspect")

    def authenticate(self) -> SessionClient:
        client = SessionClient()
        session = self.step("authenticate through local OIDC", "HTTP OIDC Authorization Code + PKCE", client.login)
        self.builder.expect(
            "oidc_principal",
            session.get("principal_id") == "northstar-demo-operator",
            "northstar-demo-operator",
            session.get("principal_id"),
        )
        return client

    def get_json(
        self,
        client: SessionClient,
        path: str,
        *,
        expected_status: int = 200,
        action_name: str | None = None,
    ) -> tuple[int, dict[str, Any], bytes]:
        status, _, body = self.step(
            action_name or f"GET {path}",
            f"HTTP GET {path}",
            lambda: client.request(path),
        )
        value = decode_json(body)
        self.builder.observations["http_statuses"][path] = status
        self.builder.expect(
            f"http_{slug(path)}_{expected_status}",
            status == expected_status,
            expected_status,
            status,
        )
        return status, value, body

    def post_json(
        self,
        client: SessionClient,
        path: str,
        body: dict[str, Any],
        *,
        expected_status: int,
        action_name: str | None = None,
    ) -> tuple[int, dict[str, Any], bytes]:
        status, _, raw = self.step(
            action_name or f"POST {path}",
            f"HTTP POST {path}",
            lambda: client.request(path, method="POST", json_body=body),
        )
        value = decode_json(raw)
        key = "POST " + path
        self.builder.observations["http_statuses"][key] = status
        self.builder.expect(
            f"http_post_{slug(path)}_{expected_status}",
            status == expected_status,
            expected_status,
            status,
        )
        return status, value, raw

    def metrics(self, client: SessionClient) -> str:
        status, headers, body = self.step(
            "read authenticated release telemetry",
            "HTTP GET /metrics",
            lambda: client.request("/metrics", accept="text/plain"),
        )
        self.builder.observations["http_statuses"]["/metrics"] = status
        self.builder.expect("metrics_http_status", status == 200, 200, status)
        content_type = header(headers, "Content-Type") or ""
        self.builder.expect(
            "metrics_content_type",
            content_type.startswith("text/plain; version=0.0.4"),
            "Prometheus text/plain; version=0.0.4",
            content_type,
        )
        text = body.decode("utf-8")
        self.builder.expect(
            "metrics_identity_redaction",
            TENANT_ID not in text and NEGATIVE_TENANT_ID not in text,
            "no tenant UUIDs",
            "redacted" if TENANT_ID not in text and NEGATIVE_TENANT_ID not in text else "tenant UUID present",
        )
        return text

    def poll_json(
        self,
        client: SessionClient,
        path: str,
        predicate: Callable[[dict[str, Any]], bool],
        *,
        timeout: float,
        description: str,
    ) -> dict[str, Any]:
        def fetch() -> dict[str, Any]:
            status, _, body = client.request(path)
            if status != 200:
                return {"_http_status": status, "_body": bounded_text(body.decode(errors="replace"))}
            return decode_json(body)

        value, attempts = self.step(
            description,
            f"bounded poll HTTP GET {path}",
            lambda: wait_until(fetch, predicate, timeout=timeout, description=description),
        )
        self.builder.observations["counts"][slug(description) + "_poll_attempts"] = attempts
        return value

    def poll_metrics(
        self,
        client: SessionClient,
        predicate: Callable[[str], bool],
        *,
        timeout: float,
        description: str,
    ) -> str:
        def fetch() -> str:
            status, _, body = client.request("/metrics", accept="text/plain")
            return body.decode("utf-8") if status == 200 else f"http_status {status}"

        value, attempts = self.step(
            description,
            "bounded poll HTTP GET /metrics",
            lambda: wait_until(fetch, predicate, timeout=timeout, description=description),
        )
        self.builder.observations["counts"][slug(description) + "_poll_attempts"] = attempts
        return value

    def db_read(self, query_name: str) -> dict[str, Any]:
        if query_name not in DB_QUERIES:
            raise GuardError(f"undeclared read-only query {query_name!r}")
        result = self.command(
            f"read PostgreSQL {query_name}",
            self.compose_command(
                "exec",
                "-T",
                "postgres",
                "psql",
                "-X",
                "-U",
                "seshatops",
                "-d",
                "seshatops_northstar_disposable",
                "-At",
                "-c",
                DB_QUERIES[query_name],
            ),
            timeout=30,
        )
        return parse_json_output(result.stdout)


def slug(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", value.lower()).strip("_")


def inventory_item(snapshot: dict[str, Any]) -> dict[str, Any] | None:
    for item in snapshot.get("items", []):
        if item.get("item_id") == ITEM_ID:
            return item
    return None


def complete_ops(value: dict[str, Any]) -> bool:
    backlog = value.get("backlog", {})
    processing = value.get("processing", {})
    return (
        backlog.get("pending") == 0
        and backlog.get("publishing") == 0
        and backlog.get("published", 0) >= 5
        and processing.get("applied", 0) + processing.get("duplicate_noop", 0) >= 5
    )


def wait_for_complete_source(driver: DemoDriver, client: SessionClient, timeout: float = 90) -> dict[str, Any]:
    return driver.poll_json(
        client,
        f"/v1/tenants/{TENANT_ID}/ops",
        complete_ops,
        timeout=timeout,
        description="wait for source outbox and projection checkpoint",
    )


def public_inventory(driver: DemoDriver, client: SessionClient) -> dict[str, Any]:
    _, snapshot, _ = driver.get_json(client, f"/v1/tenants/{TENANT_ID}/inventory")
    item = inventory_item(snapshot)
    driver.builder.expect(
        "inventory_quantity",
        item is not None and item.get("quantity_on_hand") == 8,
        8,
        None if item is None else item.get("quantity_on_hand"),
    )
    driver.builder.expect(
        "inventory_version",
        item is not None and item.get("aggregate_version") == 1,
        1,
        None if item is None else item.get("aggregate_version"),
    )
    return snapshot


def scenario_normal(driver: DemoDriver) -> None:
    client = driver.authenticate()
    stream = SSECapture(
        client,
        f"/v1/tenants/{TENANT_ID}/inventory/stream",
        "inventory_projection.updated",
    )
    driver.step("open authenticated SSE subscription", "HTTP GET inventory/stream", stream.start)
    source = driver.fixture("source")
    sse_payload = driver.step(
        "observe committed SSE projection update",
        "bounded SSE read inventory_projection.updated",
        lambda: stream.wait(60),
    )
    driver.builder.expect(
        "normal_sse_event_identity",
        sse_payload.get("last_applied_event_id") == FINAL_EVENT_ID,
        FINAL_EVENT_ID,
        sse_payload.get("last_applied_event_id"),
    )
    driver.builder.expect(
        "normal_sse_quantity",
        sse_payload.get("quantity_on_hand") == 8 and sse_payload.get("aggregate_version") == 1,
        "quantity=8 version=1",
        f"quantity={sse_payload.get('quantity_on_hand')} version={sse_payload.get('aggregate_version')}",
    )
    ops = wait_for_complete_source(driver, client)
    snapshot = public_inventory(driver, client)
    _, trace, _ = driver.get_json(
        client,
        f"/v1/tenants/{TENANT_ID}/ops/lineage/batches/{BATCH_ID}",
    )
    driver.builder.expect(
        "normal_lineage_order",
        trace.get("shipment", {}).get("order_id") == ORDER_ID,
        ORDER_ID,
        trace.get("shipment", {}).get("order_id"),
    )
    telemetry = driver.metrics(client)
    ready = metric_value(telemetry, "seshatops_runtime_ready")
    relayed = metric_value(telemetry, "seshatops_relay_publish_outcomes_total", 'outcome="published"')
    processed = metric_value(telemetry, "seshatops_consumer_processing_outcomes_total", 'outcome="processed"')
    pending = metric_value(telemetry, "seshatops_outbox_backlog_records_pending")
    driver.builder.expect("normal_runtime_ready", ready == 1, 1, ready)
    driver.builder.expect("normal_relay_telemetry", relayed >= 5, ">=5", relayed)
    driver.builder.expect("normal_consumer_telemetry", processed >= 5, ">=5", processed)
    driver.builder.expect("normal_backlog_drained", pending == 0, 0, pending)
    checkpoint = driver.checkpoint_summary()
    driver.builder.expect(
        "normal_rest_sse_checksum",
        snapshot.get("checksum") == sse_payload.get("checksum") == checkpoint.get("projection_checksum"),
        checkpoint.get("projection_checksum"),
        {"rest": snapshot.get("checksum"), "sse": sse_payload.get("checksum")},
    )
    driver.builder.observations["counts"].update(
        {
            "source": checkpoint["event_counts"]["source"],
            "published": ops["backlog"]["published"],
            "applied": ops["processing"]["applied"],
        }
    )
    driver.builder.observations["checksums"].update(
        {
            "inventory": checkpoint["projection_checksum"],
            "lineage": checkpoint["lineage_checksum"],
        }
    )
    driver.builder.observations["telemetry"].update(
        {"runtime_ready": ready, "relay_published": relayed, "consumer_processed": processed}
    )
    driver.builder.deterministic_values = {
        "fixture_version": source.get("fixture_version"),
        "event_id": FINAL_EVENT_ID,
        "inventory_checksum": checkpoint["projection_checksum"],
        "lineage_checksum": checkpoint["lineage_checksum"],
        "quantity": 8,
        "version": 1,
    }


def scenario_duplicate(driver: DemoDriver) -> None:
    client = driver.authenticate()
    driver.fixture("source")
    baseline_ops = wait_for_complete_source(driver, client)
    baseline_duplicates = baseline_ops.get("processing", {}).get("duplicate_noop", 0)
    telemetry_before = driver.metrics(client)
    processed_before = metric_value(
        telemetry_before,
        "seshatops_consumer_processing_outcomes_total",
        'outcome="processed"',
    )
    before = driver.checkpoint_summary()
    effects_before = driver.db_read("effect-counts")
    duplicate = driver.fixture("duplicate")
    ops = driver.poll_json(
        client,
        f"/v1/tenants/{TENANT_ID}/ops",
        lambda value: value.get("processing", {}).get("duplicate_noop", 0) > baseline_duplicates,
        timeout=60,
        description="wait for duplicate_noop disposition",
    )
    telemetry_after = driver.poll_metrics(
        client,
        lambda text: metric_value(
            text,
            "seshatops_consumer_processing_outcomes_total",
            'outcome="processed"',
        )
        > processed_before,
        timeout=30,
        description="wait for duplicate consumer telemetry",
    )
    processed_after = metric_value(
        telemetry_after,
        "seshatops_consumer_processing_outcomes_total",
        'outcome="processed"',
    )
    after = driver.checkpoint_summary()
    effects_after = driver.db_read("effect-counts")
    snapshot = public_inventory(driver, client)
    driver.builder.expect(
        "duplicate_inventory_checksum",
        before.get("projection_checksum") == after.get("projection_checksum") == snapshot.get("checksum"),
        before.get("projection_checksum"),
        after.get("projection_checksum"),
    )
    driver.builder.expect(
        "duplicate_lineage_checksum",
        before.get("lineage_checksum") == after.get("lineage_checksum"),
        before.get("lineage_checksum"),
        after.get("lineage_checksum"),
    )
    driver.builder.expect(
        "duplicate_effect_counts",
        effects_before == effects_after,
        effects_before,
        effects_after,
    )
    driver.builder.expect(
        "duplicate_expected_transport_behavior",
        ops["processing"]["duplicate_noop"] > baseline_duplicates,
        f">{baseline_duplicates} duplicate_noop",
        ops["processing"]["duplicate_noop"],
    )
    driver.builder.expect(
        "duplicate_consumer_telemetry_advanced",
        processed_after > processed_before,
        f">{processed_before}",
        processed_after,
    )
    driver.builder.observations["counts"].update(
        {
            **effects_after,
            "duplicate_noop_before_injection": baseline_duplicates,
            "duplicate_noop": ops["processing"]["duplicate_noop"],
        }
    )
    driver.builder.observations["checksums"].update(
        {"inventory_before": before["projection_checksum"], "inventory_after": after["projection_checksum"], "lineage_before": before["lineage_checksum"], "lineage_after": after["lineage_checksum"]}
    )
    driver.builder.observations["telemetry"].update(
        {
            "consumer_processed_before_injection": processed_before,
            "consumer_processed_after_injection": processed_after,
        }
    )
    driver.builder.deterministic_values = {
        "event_id": duplicate.get("event_id"),
        "inventory_checksum_before": before["projection_checksum"],
        "inventory_checksum_after": after["projection_checksum"],
        "lineage_checksum_before": before["lineage_checksum"],
        "lineage_checksum_after": after["lineage_checksum"],
        "effect_counts": effects_after,
        "duplicate_noop_delta": ops["processing"]["duplicate_noop"] - baseline_duplicates,
        "consumer_processed_delta": processed_after - processed_before,
    }


def scenario_poison(driver: DemoDriver) -> None:
    client = driver.authenticate()
    poison = driver.fixture("poison")
    poison_event_id = poison.get("event_id")
    ops_poison = driver.poll_json(
        client,
        f"/v1/tenants/{TENANT_ID}/ops",
        lambda value: any(
            failure.get("event_id") == poison_event_id
            and failure.get("diagnostic_code") == "unsupported_contract"
            and failure.get("quarantine_status") == "quarantined"
            for failure in value.get("processing", {}).get("failures", [])
        ),
        timeout=60,
        description="wait for incompatible event quarantine",
    )
    poison_effects = driver.db_read("poison-effects")
    driver.builder.expect(
        "poison_has_no_inbox_or_projection_effect",
        poison_effects.get("inbox_rows") == 0
        and poison_effects.get("projection_rows") == 0,
        {"inbox_rows": 0, "projection_rows": 0},
        poison_effects,
    )
    driver.builder.expect(
        "poison_has_one_attributed_failure",
        poison_effects.get("failure_rows") == 1,
        1,
        poison_effects.get("failure_rows"),
    )
    driver.fixture("source")
    wait_for_complete_source(driver, client)
    snapshot = public_inventory(driver, client)
    telemetry = driver.metrics(client)
    ready = metric_value(telemetry, "seshatops_runtime_ready")
    quarantined_failures = metric_value(
        telemetry,
        "seshatops_processing_failures_quarantined",
    )
    driver.builder.expect("poison_core_runtime_ready", ready == 1, 1, ready)
    driver.builder.expect(
        "poison_quarantine_telemetry",
        quarantined_failures == 1,
        1,
        quarantined_failures,
    )
    checkpoint = driver.checkpoint_summary()
    failure = next(
        item
        for item in ops_poison["processing"]["failures"]
        if item.get("event_id") == poison_event_id
    )
    driver.builder.expect(
        "poison_unrelated_work_continues",
        snapshot.get("checksum") == checkpoint.get("projection_checksum"),
        checkpoint.get("projection_checksum"),
        snapshot.get("checksum"),
    )
    driver.builder.observations["counts"].update(
        {
            "failures_quarantined": ops_poison["processing"]["failures_quarantined"],
            **{f"poison_{key}": value for key, value in poison_effects.items()},
        }
    )
    driver.builder.observations["checksums"].update(
        {"inventory": checkpoint["projection_checksum"], "lineage": checkpoint["lineage_checksum"]}
    )
    driver.builder.observations["telemetry"].update(
        {
            "runtime_ready": ready,
            "processing_failures_quarantined": quarantined_failures,
        }
    )
    driver.builder.deterministic_values = {
        "poison_event_id": poison_event_id,
        "diagnostic_code": failure["diagnostic_code"],
        "quarantine_status": failure["quarantine_status"],
        "processing_failures_quarantined": quarantined_failures,
        "poison_effects": poison_effects,
        "inventory_checksum": checkpoint["projection_checksum"],
        "lineage_checksum": checkpoint["lineage_checksum"],
    }


def restart_broker_and_runtime(driver: DemoDriver, *, context: str) -> SessionClient:
    driver.command(
        f"stop runtime before {context} recovery restart",
        driver.compose_command("stop", "runtime"),
        timeout=60,
    )
    driver.command(
        f"restart declared local broker for {context} recovery",
        driver.compose_command("up", "--detach", "--wait", "--wait-timeout", "120", "redpanda"),
        timeout=180,
    )
    driver.command(
        f"restart packaged runtime for {context} recovery",
        driver.compose_command("up", "--detach", "--wait", "--wait-timeout", "180", "runtime", "web"),
        timeout=240,
    )
    return driver.authenticate()


def scenario_broker_recovery(driver: DemoDriver) -> None:
    client = driver.authenticate()
    initial_metrics = driver.metrics(client)
    driver.builder.expect(
        "broker_initial_runtime_ready",
        metric_value(initial_metrics, "seshatops_runtime_ready") == 1,
        1,
        metric_value(initial_metrics, "seshatops_runtime_ready"),
    )
    driver.command("stop declared local broker", driver.compose_command("stop", "redpanda"), timeout=60)
    degraded_started = time.monotonic()
    degraded = driver.poll_metrics(
        client,
        lambda text: metric_value(text, "seshatops_runtime_ready") == 0,
        timeout=45,
        description="wait for broker degradation telemetry",
    )
    driver.fixture("source")
    backlog_ops = driver.poll_json(
        client,
        f"/v1/tenants/{TENANT_ID}/ops",
        lambda value: value.get("backlog", {}).get("pending", 0) + value.get("backlog", {}).get("publishing", 0) >= 1,
        timeout=30,
        description="wait for durable outbox backlog",
    )
    backlog_metrics = driver.metrics(client)
    pending = metric_value(backlog_metrics, "seshatops_outbox_backlog_records_pending")
    publishing = metric_value(backlog_metrics, "seshatops_outbox_backlog_records_publishing")
    driver.builder.expect("broker_degraded_readiness", metric_value(degraded, "seshatops_runtime_ready") == 0, 0, metric_value(degraded, "seshatops_runtime_ready"))
    driver.builder.expect("broker_visible_backlog", pending + publishing >= 1, ">=1", pending + publishing)
    recovery_started = time.monotonic()
    client = restart_broker_and_runtime(driver, context="broker")
    ops = wait_for_complete_source(driver, client, timeout=120)
    recovered = driver.poll_metrics(
        client,
        lambda text: metric_value(text, "seshatops_runtime_ready") == 1
        and metric_value(text, "seshatops_outbox_backlog_records_pending") == 0
        and metric_value(text, "seshatops_outbox_backlog_records_publishing") == 0,
        timeout=120,
        description="wait for broker recovery telemetry and drain",
    )
    recovery_ms = elapsed_ms(recovery_started)
    degradation_ms = elapsed_ms(degraded_started)
    snapshot = public_inventory(driver, client)
    checkpoint = driver.checkpoint_summary()
    driver.builder.expect("broker_recovered_readiness", metric_value(recovered, "seshatops_runtime_ready") == 1, 1, metric_value(recovered, "seshatops_runtime_ready"))
    driver.builder.expect("broker_recovered_backlog", ops["backlog"]["pending"] == 0 and ops["backlog"]["publishing"] == 0, "pending=0 publishing=0", ops["backlog"])
    driver.builder.expect("broker_recovered_projection", snapshot.get("checksum") == checkpoint.get("projection_checksum"), checkpoint.get("projection_checksum"), snapshot.get("checksum"))
    driver.builder.observations["durations_ms"].update(
        {"observed_degradation_window": degradation_ms, "observed_restart_to_drain": recovery_ms}
    )
    driver.builder.observations["counts"].update(
        {"backlog_during_interruption": int(pending + publishing), "published_after_recovery": ops["backlog"]["published"]}
    )
    driver.builder.observations["checksums"].update(
        {"inventory": checkpoint["projection_checksum"], "lineage": checkpoint["lineage_checksum"]}
    )
    driver.builder.observations["telemetry"].update(
        {
            "runtime_ready_before": metric_value(initial_metrics, "seshatops_runtime_ready"),
            "runtime_ready_degraded": metric_value(degraded, "seshatops_runtime_ready"),
            "outbox_pending_degraded": pending,
            "outbox_publishing_degraded": publishing,
            "runtime_ready_recovered": metric_value(recovered, "seshatops_runtime_ready"),
            "outbox_pending_recovered": metric_value(
                recovered, "seshatops_outbox_backlog_records_pending"
            ),
            "outbox_publishing_recovered": metric_value(
                recovered, "seshatops_outbox_backlog_records_publishing"
            ),
        }
    )
    driver.builder.deterministic_values = {
        "inventory_checksum": checkpoint["projection_checksum"],
        "lineage_checksum": checkpoint["lineage_checksum"],
        "final_readiness": 1,
        "final_pending": 0,
    }


def scenario_rebuild(driver: DemoDriver) -> None:
    client = driver.authenticate()
    driver.fixture("source")
    wait_for_complete_source(driver, client)
    before = driver.checkpoint_summary()
    _, result, _ = driver.post_json(
        client,
        f"/v1/tenants/{TENANT_ID}/ops/rebuild",
        {},
        expected_status=200,
    )
    driver.builder.expect("rebuild_complete", result.get("status") == "complete", "complete", result.get("status"))
    driver.builder.expect("rebuild_inventory_checksum", result.get("checksum") == before.get("projection_checksum"), before.get("projection_checksum"), result.get("checksum"))
    driver.builder.expect("rebuild_lineage_checksum", result.get("lineage_checksum") == before.get("lineage_checksum"), before.get("lineage_checksum"), result.get("lineage_checksum"))
    driver.builder.expect(
        "rebuild_replayed_complete_fixture",
        result.get("applied") == 5
        and result.get("duplicate_noop") == 0
        and result.get("quarantined") == 0,
        {"applied": 5, "duplicate_noop": 0, "quarantined": 0},
        {
            "applied": result.get("applied"),
            "duplicate_noop": result.get("duplicate_noop"),
            "quarantined": result.get("quarantined"),
        },
    )
    after = driver.checkpoint_summary()
    snapshot = public_inventory(driver, client)
    driver.builder.expect("rebuild_after_inventory_checksum", after.get("projection_checksum") == before.get("projection_checksum") == snapshot.get("checksum"), before.get("projection_checksum"), after.get("projection_checksum"))
    driver.builder.expect("rebuild_after_lineage_checksum", after.get("lineage_checksum") == before.get("lineage_checksum"), before.get("lineage_checksum"), after.get("lineage_checksum"))
    _, audit, _ = driver.get_json(client, f"/v1/tenants/{TENANT_ID}/ops/audit")
    driver.builder.expect(
        "rebuild_audit_allow",
        any(record.get("resource") == "RES-REBUILD" and record.get("outcome") == "allow" for record in audit.get("records", [])),
        "append-only allow decision for RES-REBUILD",
        audit.get("records", []),
    )
    telemetry = driver.metrics(client)
    control_labels = 'operation="rebuild",outcome="complete"'
    rebuild_complete = metric_value(
        telemetry,
        "seshatops_control_operations_total",
        control_labels,
    )
    rebuild_duration_count = metric_value(
        telemetry,
        "seshatops_control_duration_seconds_count",
        control_labels,
    )
    rebuild_duration_sum = metric_value(
        telemetry,
        "seshatops_control_duration_seconds_sum",
        control_labels,
    )
    driver.builder.expect("rebuild_complete_telemetry", rebuild_complete == 1, 1, rebuild_complete)
    driver.builder.expect(
        "rebuild_duration_telemetry",
        rebuild_duration_count == 1 and rebuild_duration_sum >= 0,
        "count=1 and non-negative duration sum",
        {"count": rebuild_duration_count, "sum": rebuild_duration_sum},
    )
    driver.builder.observations["counts"].update(
        {"applied": result.get("applied"), "duplicate_noop": result.get("duplicate_noop"), "quarantined": result.get("quarantined")}
    )
    driver.builder.observations["checksums"].update(
        {"inventory_before": before["projection_checksum"], "inventory_after": after["projection_checksum"], "lineage_before": before["lineage_checksum"], "lineage_after": after["lineage_checksum"]}
    )
    driver.builder.observations["telemetry"].update(
        {
            "rebuild_complete": rebuild_complete,
            "rebuild_duration_count": rebuild_duration_count,
            "rebuild_duration_sum": rebuild_duration_sum,
        }
    )
    driver.builder.deterministic_values = {
        "status": result.get("status"),
        "inventory_checksum": result.get("checksum"),
        "lineage_checksum": result.get("lineage_checksum"),
        "control_complete_count": rebuild_complete,
        "control_duration_count": rebuild_duration_count,
        "replay_counts": {
            "applied": result.get("applied"),
            "duplicate_noop": result.get("duplicate_noop"),
            "quarantined": result.get("quarantined"),
        },
    }


def scenario_tenant_isolation(driver: DemoDriver) -> None:
    client = driver.authenticate()
    driver.fixture("source")
    wait_for_complete_source(driver, client)
    allowed_before = public_inventory(driver, client)
    checkpoint_before = driver.checkpoint_summary()
    foreign_before = driver.db_read("foreign-effects")
    audit_before = driver.db_read("foreign-deny-audits")
    allowed_audit_before = driver.db_read("allowed-rebuild-audits")
    _, read_error, read_raw = driver.get_json(
        client,
        f"/v1/tenants/{NEGATIVE_TENANT_ID}/inventory",
        expected_status=403,
    )
    _, mutation_error, mutation_raw = driver.post_json(
        client,
        f"/v1/tenants/{NEGATIVE_TENANT_ID}/ops/rebuild",
        {"tenant_id": TENANT_ID},
        expected_status=403,
    )
    driver.builder.expect("tenant_read_minimal_error", read_error == {"error": "forbidden"}, {"error": "forbidden"}, read_error)
    driver.builder.expect("tenant_mutation_minimal_error", mutation_error == {"error": "forbidden"}, {"error": "forbidden"}, mutation_error)
    leaked_markers = (TENANT_ID, ITEM_ID, FINAL_EVENT_ID, allowed_before.get("checksum", ""))
    combined = read_raw.decode(errors="replace") + mutation_raw.decode(errors="replace")
    leaked = [marker for marker in leaked_markers if marker and marker in combined]
    driver.builder.expect("tenant_no_payload_leak", not leaked, "no allowed-tenant payload markers", leaked)
    foreign_after = driver.db_read("foreign-effects")
    audit_after = driver.db_read("foreign-deny-audits")
    allowed_audit_after = driver.db_read("allowed-rebuild-audits")
    allowed_after = public_inventory(driver, client)
    checkpoint_after = driver.checkpoint_summary()
    driver.builder.expect("tenant_no_foreign_effect", foreign_after == foreign_before, foreign_before, foreign_after)
    driver.builder.expect(
        "tenant_mutation_deny_audited",
        audit_after.get("rebuild_denials") == audit_before.get("rebuild_denials", 0) + 1,
        audit_before.get("rebuild_denials", 0) + 1,
        audit_after.get("rebuild_denials"),
    )
    driver.builder.expect("tenant_no_allowed_effect", allowed_after.get("checksum") == allowed_before.get("checksum"), allowed_before.get("checksum"), allowed_after.get("checksum"))
    driver.builder.expect(
        "tenant_no_allowed_rebuild_audit",
        allowed_audit_after == allowed_audit_before,
        allowed_audit_before,
        allowed_audit_after,
    )
    driver.builder.expect(
        "tenant_allowed_checkpoint_unchanged",
        checkpoint_after.get("projection_checksum") == checkpoint_before.get("projection_checksum")
        and checkpoint_after.get("lineage_checksum") == checkpoint_before.get("lineage_checksum"),
        {
            "inventory": checkpoint_before.get("projection_checksum"),
            "lineage": checkpoint_before.get("lineage_checksum"),
        },
        {
            "inventory": checkpoint_after.get("projection_checksum"),
            "lineage": checkpoint_after.get("lineage_checksum"),
        },
    )
    metrics = driver.metrics(client)
    inventory_denials = metric_value(metrics, "seshatops_auth_denials_total", 'route="inventory",reason="forbidden"')
    ops_denials = metric_value(metrics, "seshatops_auth_denials_total", 'route="ops",reason="forbidden"')
    rebuild_denied = metric_value(
        metrics,
        "seshatops_control_operations_total",
        'operation="rebuild",outcome="denied"',
    )
    rebuild_complete = metric_value(
        metrics,
        "seshatops_control_operations_total",
        'operation="rebuild",outcome="complete"',
    )
    driver.builder.expect("tenant_inventory_denial_telemetry", inventory_denials == 1, 1, inventory_denials)
    driver.builder.expect("tenant_ops_denial_telemetry", ops_denials == 1, 1, ops_denials)
    driver.builder.expect("tenant_control_denial_telemetry", rebuild_denied == 1, 1, rebuild_denied)
    driver.builder.expect("tenant_no_control_complete_telemetry", rebuild_complete == 0, 0, rebuild_complete)
    driver.builder.observations["counts"].update(
        {
            "inventory_denials": int(inventory_denials),
            "ops_denials": int(ops_denials),
            **foreign_after,
            **audit_after,
            **allowed_audit_after,
        }
    )
    driver.builder.observations["checksums"].update(
        {
            "allowed_inventory_before": checkpoint_before["projection_checksum"],
            "allowed_inventory_after": checkpoint_after["projection_checksum"],
            "allowed_lineage_before": checkpoint_before["lineage_checksum"],
            "allowed_lineage_after": checkpoint_after["lineage_checksum"],
        }
    )
    driver.builder.observations["telemetry"].update(
        {
            "inventory_denials": inventory_denials,
            "ops_denials": ops_denials,
            "rebuild_denied": rebuild_denied,
            "rebuild_complete": rebuild_complete,
        }
    )
    driver.builder.deterministic_values = {
        "read_status": 403,
        "mutation_status": 403,
        "foreign_effects": foreign_after,
        "rebuild_deny_audits": audit_after.get("rebuild_denials"),
        "rebuild_allow_audits": allowed_audit_after.get("rebuild_allows"),
        "allowed_checksum": allowed_after["checksum"],
        "allowed_lineage_checksum": checkpoint_after["lineage_checksum"],
        "telemetry": {
            "inventory_denials": inventory_denials,
            "ops_denials": ops_denials,
            "rebuild_denied": rebuild_denied,
            "rebuild_complete": rebuild_complete,
        },
    }


def feature_snapshot(driver: DemoDriver, client: SessionClient, label: str) -> dict[str, Any]:
    _, value, _ = driver.get_json(
        client,
        f"/v1/tenants/{TENANT_ID}/forecast/features",
        action_name=f"read {label} forecast-source state",
    )
    return value


def expect_noncomplete_snapshot(driver: DemoDriver, name: str, snapshot: dict[str, Any], status: str) -> None:
    driver.builder.expect(f"forecast_{name}_status", snapshot.get("status") == status, status, snapshot.get("status"))
    driver.builder.expect(f"forecast_{name}_rows", snapshot.get("rows") == [], [], snapshot.get("rows"))
    driver.builder.expect(f"forecast_{name}_reason", bool(snapshot.get("status_reasons")), "at least one bounded reason", snapshot.get("status_reasons"))
    for field in ("snapshot_id", "checksum"):
        value = snapshot.get(field)
        driver.builder.expect(
            f"forecast_{name}_{field}",
            isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value) is not None,
            "64-character lowercase SHA-256",
            value,
        )


def scenario_forecast_states(driver: DemoDriver) -> None:
    client = driver.authenticate()
    insufficient = feature_snapshot(driver, client, "insufficient")
    expect_noncomplete_snapshot(driver, "insufficient", insufficient, "insufficient")
    driver.command(
        "stop declared local broker for stale source",
        driver.compose_command("stop", "redpanda"),
        timeout=60,
    )
    driver.fixture("source")
    stale = driver.poll_json(
        client,
        f"/v1/tenants/{TENANT_ID}/forecast/features",
        lambda value: value.get("status") == "stale",
        timeout=30,
        description="wait for stale forecast-source state",
    )
    expect_noncomplete_snapshot(driver, "stale", stale, "stale")
    client = restart_broker_and_runtime(driver, context="stale-source")
    wait_for_complete_source(driver, client, timeout=120)
    incomplete_fault = driver.fixture("forecast-incomplete")
    incomplete = driver.poll_json(
        client,
        f"/v1/tenants/{TENANT_ID}/forecast/features",
        lambda value: value.get("status") == "incomplete",
        timeout=30,
        description="wait for incomplete forecast-source state",
    )
    expect_noncomplete_snapshot(driver, "incomplete", incomplete, "incomplete")
    driver.builder.expect(
        "forecast_incomplete_reason_is_retained_source",
        any("malformed retained event" in reason for reason in incomplete.get("status_reasons", [])),
        "malformed retained event reason",
        incomplete.get("status_reasons"),
    )
    metrics = driver.metrics(client)
    ready = metric_value(metrics, "seshatops_runtime_ready")
    driver.builder.expect("forecast_runtime_ready", ready == 1, 1, ready)
    driver.builder.observations["counts"].update(
        {
            "insufficient_rows": len(insufficient["rows"]),
            "stale_rows": len(stale["rows"]),
            "incomplete_rows": len(incomplete["rows"]),
        }
    )
    driver.builder.observations["checksums"].update(
        {
            "insufficient_snapshot": insufficient.get("checksum"),
            "stale_snapshot": stale.get("checksum"),
            "incomplete_snapshot": incomplete.get("checksum"),
        }
    )
    driver.builder.observations["telemetry"]["runtime_ready"] = ready
    driver.builder.deterministic_values = {
        "statuses": [insufficient["status"], stale["status"], incomplete["status"]],
        "snapshot_ids": [insufficient.get("snapshot_id"), stale.get("snapshot_id"), incomplete.get("snapshot_id")],
        "fault_event_id": incomplete_fault.get("event_id"),
    }


def structured_outcome(stderr: str, expected: str) -> bool:
    for line in stderr.splitlines():
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if (
            isinstance(value, dict)
            and (value.get("msg") or value.get("event")) == "forecast.command.failed"
            and value.get("outcome") == expected
        ):
            return True
    return False


def scenario_python_degradation(driver: DemoDriver) -> None:
    client = driver.authenticate()
    driver.builder.observations["fixture_versions"]["forecast_history"] = FORECAST_HISTORY_FIXTURE_VERSION
    before = driver.db_read("prediction-count")
    driver.builder.expect("python_initial_prediction_count", before.get("prediction_rows") == 0, 0, before.get("prediction_rows"))
    _, missing_prediction, _ = driver.get_json(
        client,
        f"/v1/tenants/{TENANT_ID}/forecast/predictions/{ITEM_ID}",
        expected_status=404,
    )
    driver.builder.expect("python_initial_prediction_404", missing_prediction == {"error": "not_found"}, {"error": "not_found"}, missing_prediction)

    unavailable = driver.command(
        "run forecast with Python unavailable",
        driver.compose_command(
            "exec",
            "-T",
            "-e",
            f"SESHATOPS_FORECAST_CONFIRM={FORECAST_CONFIRMATION}",
            "-e",
            "SESHATOPS_FORECAST_PYTHON=/missing/seshatops-python",
            "runtime",
            "/app/seshatops",
            "forecast",
        ),
        timeout=30,
        expected_codes={1},
    )
    driver.builder.expect("python_unavailable_nonzero", unavailable.returncode != 0, "non-zero", unavailable.returncode)
    driver.builder.expect("python_unavailable_telemetry", structured_outcome(unavailable.stderr, "unavailable"), "forecast.command.failed outcome=unavailable", bounded_text(unavailable.stderr))
    after_unavailable = driver.db_read("prediction-count")
    driver.builder.expect("python_unavailable_no_write", after_unavailable == before, before, after_unavailable)

    timed_out = driver.command(
        "run forecast with Python timeout",
        driver.compose_command(
            "exec",
            "-T",
            "-e",
            f"SESHATOPS_FORECAST_CONFIRM={FORECAST_CONFIRMATION}",
            "-e",
            "SESHATOPS_FORECAST_TIMEOUT=5s",
            "-e",
            "SESHATOPS_FORECAST_CANDIDATE=/app/scripts/demo-forecast-timeout.py",
            "runtime",
            "/app/seshatops",
            "forecast",
        ),
        timeout=30,
        expected_codes={1},
    )
    driver.builder.expect("python_timeout_nonzero", timed_out.returncode != 0, "non-zero", timed_out.returncode)
    driver.builder.expect("python_timeout_telemetry", structured_outcome(timed_out.stderr, "timeout"), "forecast.command.failed outcome=timeout", bounded_text(timed_out.stderr))
    after_timeout = driver.db_read("prediction-count")
    driver.builder.expect("python_timeout_no_write", after_timeout == before, before, after_timeout)

    driver.fixture("source")
    wait_for_complete_source(driver, client)
    snapshot = public_inventory(driver, client)
    metrics = driver.metrics(client)
    ready = metric_value(metrics, "seshatops_runtime_ready")
    driver.builder.expect("python_failure_core_ready", ready == 1, 1, ready)
    final_count = driver.db_read("prediction-count")
    driver.builder.expect("python_failure_final_no_write", final_count == before, before, final_count)
    driver.builder.observations["counts"].update(
        {"predictions_before": before["prediction_rows"], "predictions_after": final_count["prediction_rows"]}
    )
    driver.builder.observations["checksums"]["core_inventory"] = snapshot["checksum"]
    driver.builder.observations["telemetry"].update(
        {"python_unavailable": "unavailable", "python_timeout": "timeout", "runtime_ready": ready}
    )
    driver.builder.deterministic_values = {
        "unavailable_outcome": "unavailable",
        "timeout_outcome": "timeout",
        "prediction_rows": final_count["prediction_rows"],
        "core_inventory_checksum": snapshot["checksum"],
    }


SCENARIOS: dict[str, Callable[[DemoDriver], None]] = {
    "normal-flow": scenario_normal,
    "duplicate-delivery": scenario_duplicate,
    "poison-isolation": scenario_poison,
    "broker-recovery": scenario_broker_recovery,
    "deterministic-rebuild": scenario_rebuild,
    "tenant-isolation": scenario_tenant_isolation,
    "forecast-source-states": scenario_forecast_states,
    "python-degradation": scenario_python_degradation,
}


def failure_object(error: BaseException) -> dict[str, str]:
    category = "unexpected"
    if isinstance(error, GuardError):
        category = "guard"
    elif isinstance(error, (CommandTimeout, ScenarioTimeout)):
        category = "timeout"
    elif isinstance(error, CommandFailure):
        category = "command"
    elif isinstance(error, InvariantFailure):
        category = "invariant"
    return {"category": category, "message": bounded_text(str(error))}


def capture_diagnostics(driver: DemoDriver, scenario: str) -> list[str]:
    diagnostics: list[str] = []
    for suffix, args in (
        ("ps.json", driver.compose_command("ps", "--format", "json")),
        (
            "runtime-redpanda.log",
            driver.compose_command("logs", "--no-color", "--tail", "200", "runtime", "redpanda"),
        ),
    ):
        try:
            result = driver.runner.run(args, timeout=30)
            raw = redact_command_output(result.stdout + result.stderr, args).encode()[
                :MAX_DIAGNOSTIC_BYTES
            ]
            path = driver.evidence_dir / f"{scenario}.{suffix}"
            path.write_bytes(raw)
        except Exception:
            continue
        diagnostics.append(path.name)
    return diagnostics


def cleanup_environment(driver: DemoDriver) -> dict[str, Any]:
    if not driver.guarded:
        return {"attempted": False, "status": "not_run", "actions": []}
    started_at = utc_now()
    args = driver.compose_command("down", "--volumes", "--remove-orphans")
    try:
        result = driver.runner.run(
            args,
            timeout=120,
        )
    except Exception as error:
        return {
            "attempted": True,
            "status": "failed",
            "actions": [{"command": "docker compose down --volumes --remove-orphans", "started_at": started_at, "status": "failed"}],
            "error": bounded_text(str(error) or error.__class__.__name__),
        }
    status = "passed" if result.returncode == 0 else "failed"
    cleanup = {
        "attempted": True,
        "status": status,
        "actions": [
            {
                "command": "docker compose down --volumes --remove-orphans",
                "started_at": started_at,
                "duration_ms": result.duration_ms,
                "exit_code": result.returncode,
                "status": status,
            }
        ],
    }
    if status == "failed":
        detail = result.stderr or result.stdout or "cleanup failed without output"
        cleanup["error"] = bounded_text(redact_command_output(detail, args))
    return cleanup


def run_one(
    scenario: str,
    *,
    release: dict[str, Any],
    evidence_dir: Path,
    runner: CommandRunner,
    break_expectation: str | None = None,
) -> dict[str, Any]:
    builder = ResultBuilder(scenario, release, break_expectation=break_expectation)
    driver = DemoDriver(runner, builder, evidence_dir)
    error: BaseException | None = None
    diagnostics: list[str] = []
    try:
        driver.verify_release_identity(release, "before_build")
        driver.fresh_start(release)
        SCENARIOS[scenario](driver)
    except BaseException as caught:
        error = caught
    try:
        driver.verify_release_identity(release, "after_scenario")
    except BaseException as caught:
        if error is None:
            error = caught
    if error is not None and driver.guarded:
        try:
            diagnostics = capture_diagnostics(driver, scenario)
        except Exception:
            diagnostics = []
    cleanup = cleanup_environment(driver)
    snapshot_error = driver.remove_compose_snapshot()
    if snapshot_error is not None:
        cleanup["status"] = "failed"
        cleanup["error"] = snapshot_error
        cleanup.setdefault("actions", []).append(
            {
                "command": "remove pinned Compose snapshot",
                "started_at": utc_now(),
                "status": "failed",
            }
        )
    if cleanup["status"] == "failed" and error is None:
        error = DemoError(cleanup.get("error", "disposable environment cleanup failed"))
    status = "passed" if error is None else "failed"
    result = builder.finish(
        status=status,
        cleanup=cleanup,
        failure=None if error is None else failure_object(error),
        diagnostics=diagnostics,
    )
    path = evidence_dir / f"{scenario}.json"
    path.write_text(json.dumps(result, indent=2, sort_keys=True, allow_nan=False) + "\n")
    summary_path = evidence_dir / f"{scenario}.txt"
    summary_path.write_text(render_human_summary(result))
    summary_bits = [f"{result['status'].upper()} {scenario}", f"duration={result['duration_ms']}ms"]
    checksums = result["observations"].get("checksums", {})
    if checksums:
        summary_bits.append("checksums=" + ",".join(f"{key}:{str(value)[:12]}" for key, value in sorted(checksums.items())))
    if result["failure"] is not None:
        summary_bits.append("failure=" + result["failure"]["message"])
    print(" | ".join(summary_bits))
    print(f"  evidence: {path} (human summary: {summary_path.name})")
    return result


def render_human_summary(result: dict[str, Any]) -> str:
    release = result["release"]
    lines = [
        f"SeshatOps release demonstration: {result['scenario']}",
        f"Result: {result['status'].upper()}",
        f"Release version: {release['version']}",
        f"Release commit: {release['commit']}",
        f"Release source SHA-256: {release['source_sha256']}",
        f"Worktree dirty: {str(release['worktree_dirty']).lower()}",
        f"Harness version: {release['harness_version']}",
        f"Fixture version: {result['fixture_version']}",
        f"Started: {result['started_at']}",
        f"Finished: {result['finished_at']}",
        f"Duration: {result['duration_ms']} ms",
        "",
        "Preconditions:",
    ]
    lines.extend(f"- {precondition}" for precondition in result["preconditions"])
    lines.extend([
        "",
        "Actions:",
    ])
    for action in result["actions"]:
        exit_text = f" exit={action['exit_code']}" if "exit_code" in action else ""
        lines.append(
            f"- {action['status'].upper()} {action['name']} ({action['duration_ms']} ms{exit_text}): {action['command']}"
        )
    lines.extend(["", "Expected outcomes:"])
    lines.extend(f"- {outcome}" for outcome in result["expected_outcomes"])
    lines.extend(["", "Observed expectations:"])
    for expectation in result["expectations"]:
        marker = "PASS" if expectation["passed"] else "FAIL"
        lines.append(
            f"- {marker} {expectation['name']}: expected={expectation['expected']!r} observed={expectation['observed']!r}"
        )
    lines.extend(["", "Key observations:"])
    for group in (
        "fixture_versions",
        "http_statuses",
        "counts",
        "durations_ms",
        "checksums",
        "telemetry",
    ):
        values = result["observations"].get(group, {})
        if values:
            lines.append(f"- {group}: {json.dumps(values, sort_keys=True, separators=(',', ':'))}")
    lines.append(f"- deterministic identity: {result['deterministic_identity']['sha256']}")
    if result["failure"] is not None:
        lines.extend(
            [
                "",
                "Failure:",
                f"- category: {result['failure']['category']}",
                f"- message: {result['failure']['message']}",
            ]
        )
    lines.extend(["", "Cleanup:", f"- status: {result['cleanup']['status']}"])
    for action in result["cleanup"].get("actions", []):
        duration_text = (
            f" duration={action['duration_ms']}ms" if "duration_ms" in action else ""
        )
        exit_text = f" exit={action['exit_code']}" if "exit_code" in action else ""
        lines.append(
            f"- {action['status'].upper()} {action['command']}"
            f"{duration_text}{exit_text} at {action['started_at']}"
        )
    if result["cleanup"].get("error"):
        lines.append(f"- error: {result['cleanup']['error']}")
    if result["diagnostics"]:
        lines.extend(["", "Diagnostics:"])
        lines.extend(f"- {diagnostic}" for diagnostic in result["diagnostics"])
    lines.extend(["", "Known limitations:"])
    lines.extend(f"- {limitation}" for limitation in result["limitations"])
    return "\n".join(lines) + "\n"


def campaign_result(
    release: dict[str, Any],
    started_at: str,
    started_monotonic: float,
    results: list[dict[str, Any]],
    comparison: dict[str, Any] | None,
) -> dict[str, Any]:
    status = "passed" if len(results) == len(SCENARIO_ORDER) and all(item["status"] == "passed" for item in results) else "failed"
    identities = {item["scenario"]: item["deterministic_identity"] for item in results}
    failure = None
    if status == "failed":
        failed = next((item for item in results if item["status"] == "failed"), None)
        failure = failed["failure"] if failed else {"category": "campaign", "message": "campaign did not run every scenario"}
    result = {
        "schema_version": SCHEMA_VERSION,
        "kind": "campaign_result",
        "release": release,
        "fixture_version": FIXTURE_VERSION,
        "started_at": started_at,
        "finished_at": utc_now(),
        "duration_ms": elapsed_ms(started_monotonic),
        "status": status,
        "failure": failure,
        "scenarios": [
            {
                "scenario": item["scenario"],
                "status": item["status"],
                "result_file": item["scenario"] + ".json",
                "deterministic_identity": item["deterministic_identity"],
            }
            for item in results
        ],
        "deterministic_identities": identities,
        "comparison": comparison,
        "limitations": list(COMMON_LIMITATIONS),
    }
    try:
        encoded = json.dumps(result, sort_keys=True, allow_nan=False)
    except (TypeError, ValueError) as error:
        raise InvariantFailure("campaign result contains a non-JSON value") from error
    if len(encoded.encode()) > MAX_RESULT_BYTES:
        raise InvariantFailure("campaign result exceeds the bounded 256 KiB limit")
    return result


def validate_comparable_campaign(value: dict[str, Any], *, label: str) -> None:
    try:
        json.dumps(value, allow_nan=False)
    except (TypeError, ValueError) as error:
        raise InvariantFailure(f"{label} contains a non-JSON value") from error
    if value.get("kind") != "campaign_result" or value.get("schema_version") != SCHEMA_VERSION:
        raise InvariantFailure(f"{label} is not a release-demo campaign result")
    if value.get("status") != "passed" or value.get("failure") is not None:
        raise InvariantFailure(f"{label} is not a passed campaign")
    release = value.get("release")
    if (
        not isinstance(release, dict)
        or not release.get("commit")
        or not release.get("version")
        or not release.get("harness_version")
        or not re.fullmatch(r"[0-9a-f]{64}", release.get("source_sha256", ""))
        or release.get("worktree_dirty") is not False
    ):
        raise InvariantFailure(f"{label} does not identify a clean release checkout")
    scenarios = value.get("scenarios")
    if not isinstance(scenarios, list) or [item.get("scenario") for item in scenarios if isinstance(item, dict)] != list(SCENARIO_ORDER):
        raise InvariantFailure(f"{label} does not contain all scenarios in campaign order")
    if any(not isinstance(item, dict) or item.get("status") != "passed" for item in scenarios):
        raise InvariantFailure(f"{label} contains a failed scenario")
    identities = value.get("deterministic_identities")
    if not isinstance(identities, dict) or set(identities) != set(SCENARIO_ORDER):
        raise InvariantFailure(f"{label} does not contain all deterministic identities")
    for scenario, identity in identities.items():
        if not isinstance(identity, dict) or not re.fullmatch(r"[0-9a-f]{64}", identity.get("sha256", "")):
            raise InvariantFailure(f"{label} has an invalid deterministic identity for {scenario}")
    if any(
        item.get("deterministic_identity") != identities[item["scenario"]]
        for item in scenarios
    ):
        raise InvariantFailure(f"{label} scenario identities do not match its identity index")


def compare_campaign(previous_path: Path, current: dict[str, Any]) -> dict[str, Any]:
    try:
        flags = os.O_RDONLY | getattr(os, "O_NONBLOCK", 0) | getattr(os, "O_NOFOLLOW", 0)
        descriptor = os.open(previous_path, flags)
        try:
            metadata = os.fstat(descriptor)
            if not stat.S_ISREG(metadata.st_mode):
                raise InvariantFailure("comparison campaign is not a regular file")
            with os.fdopen(descriptor, "rb") as handle:
                descriptor = -1
                raw = handle.read(MAX_RESULT_BYTES + 1)
        finally:
            if descriptor >= 0:
                os.close(descriptor)
        if len(raw) > MAX_RESULT_BYTES:
            raise InvariantFailure("comparison campaign exceeds the bounded 256 KiB limit")
        previous = json.loads(
            raw.decode("utf-8"),
            parse_constant=lambda value: (_ for _ in ()).throw(
                ValueError(f"non-standard JSON constant {value}")
            ),
        )
    except (OSError, UnicodeDecodeError, ValueError) as error:
        raise InvariantFailure("could not read <prior-campaign>") from error
    if not isinstance(previous, dict):
        raise InvariantFailure("comparison file is not a JSON object")
    validate_comparable_campaign(previous, label="comparison campaign")
    validate_comparable_campaign(current, label="current campaign")
    if previous.get("release") != current.get("release"):
        raise InvariantFailure("campaign comparison release identities differ")
    if previous.get("fixture_version") != current.get("fixture_version"):
        raise InvariantFailure("campaign comparison fixture versions differ")
    expected = previous.get("deterministic_identities")
    observed = current.get("deterministic_identities")
    if expected != observed:
        raise InvariantFailure("campaign deterministic identities differ from the comparison run")
    return {
        "status": "matched",
        "previous": "<prior-campaign>",
        "release_commit": current["release"]["commit"],
        "scenario_count": len(observed),
    }


def prepare_evidence_dir(path: Path) -> None:
    try:
        if path.exists():
            if not path.is_dir():
                raise DemoError("evidence path must be a directory")
            if next(path.iterdir(), None) is not None:
                raise DemoError("evidence directory must be new or empty")
        path.mkdir(parents=True, exist_ok=True)
    except DemoError:
        raise
    except OSError as error:
        detail = bounded_text(error.strerror or type(error).__name__)
        raise DemoError(f"evidence directory could not be prepared: {detail}") from error


def validate_evidence_dir(path: Path) -> None:
    try:
        path.relative_to(ROOT.resolve())
    except ValueError:
        return
    allowed = (ROOT / ".release-evidence").resolve()
    if path != allowed and allowed not in path.parents:
        raise DemoError(
            "evidence inside the repository must stay below .release-evidence"
        )


def default_evidence_dir() -> Path:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return ROOT / ".release-evidence" / stamp


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Run bounded demonstrations against the packaged disposable local stack.",
    )
    parser.add_argument("scenario", choices=("all",) + SCENARIO_ORDER)
    parser.add_argument("--evidence-dir", type=Path, default=None)
    parser.add_argument("--compare", type=Path, default=None, help="Compare a full campaign with a prior campaign.json.")
    parser.add_argument(
        "--break-expectation",
        default=None,
        help="Verification-only hook: force one named expectation to fail.",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.compare is not None and args.scenario != "all":
        parser.error("--compare is valid only with the all campaign")
    if args.break_expectation is not None and args.scenario == "all":
        parser.error("--break-expectation requires one named scenario")
    evidence_dir = (args.evidence_dir or default_evidence_dir()).resolve()
    try:
        validate_evidence_dir(evidence_dir)
        prepare_evidence_dir(evidence_dir)
    except DemoError as error:
        print(f"release demo failed: {error}", file=sys.stderr)
        return 2
    runner = CommandRunner()
    try:
        release = release_metadata(runner)
    except (DemoError, OSError) as error:
        print(
            f"release demo failed: {bounded_text(str(error))}",
            file=sys.stderr,
        )
        return 2

    if args.scenario != "all":
        result = run_one(
            args.scenario,
            release=release,
            evidence_dir=evidence_dir,
            runner=runner,
            break_expectation=args.break_expectation,
        )
        return 0 if result["status"] == "passed" else 1

    campaign_started_at = utc_now()
    campaign_started = time.monotonic()
    results: list[dict[str, Any]] = []
    for scenario in SCENARIO_ORDER:
        result = run_one(
            scenario,
            release=release,
            evidence_dir=evidence_dir,
            runner=runner,
            break_expectation=args.break_expectation,
        )
        results.append(result)
        if result["status"] == "failed":
            break
    provisional = campaign_result(release, campaign_started_at, campaign_started, results, None)
    comparison = None
    comparison_error: BaseException | None = None
    if provisional["status"] == "passed":
        try:
            require_release_unchanged(runner, release)
            if args.compare is not None:
                comparison = compare_campaign(args.compare.resolve(), provisional)
        except BaseException as error:
            comparison_error = error
    final = campaign_result(release, campaign_started_at, campaign_started, results, comparison)
    if comparison_error is not None:
        final["status"] = "failed"
        final["failure"] = failure_object(comparison_error)
    path = evidence_dir / "campaign.json"
    path.write_text(json.dumps(final, indent=2, sort_keys=True, allow_nan=False) + "\n")
    print(
        f"{final['status'].upper()} full campaign | scenarios={len(results)}/{len(SCENARIO_ORDER)} "
        f"| duration={final['duration_ms']}ms"
    )
    if comparison is not None:
        print(f"  deterministic comparison: {comparison['status']} ({comparison['scenario_count']} scenarios)")
    if final["failure"] is not None:
        print(f"  failure: {final['failure']['message']}")
    print(f"  evidence: {path}")
    return 0 if final["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
