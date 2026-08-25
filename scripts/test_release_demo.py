#!/usr/bin/env python3
"""Unit tests for the bounded local release demonstration harness."""

from __future__ import annotations

import copy
import importlib.util
import io
import json
import subprocess
import sys
import tempfile
import threading
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).with_name("release_demo.py")
SPEC = importlib.util.spec_from_file_location("seshatops_release_demo", SCRIPT)
if SPEC is None or SPEC.loader is None:  # pragma: no cover - import setup failure
    raise RuntimeError(f"could not load {SCRIPT}")
release_demo = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = release_demo
SPEC.loader.exec_module(release_demo)


RELEASE = {
    "version": "v0.0.0-test",
    "commit": "0123456789abcdef",
    "worktree_dirty": False,
    "source_sha256": "a" * 64,
    "harness_version": release_demo.HARNESS_VERSION,
}


def valid_compose_config() -> dict:
    return {
        "name": release_demo.PROJECT_NAME,
        "networks": {
            "local": {
                "name": f"{release_demo.PROJECT_NAME}_local",
                "ipam": {},
            }
        },
        "volumes": {
            "postgres-data": {
                "name": f"{release_demo.PROJECT_NAME}_postgres-data",
            },
            "redpanda-data": {
                "name": f"{release_demo.PROJECT_NAME}_redpanda-data",
            },
        },
        "services": {
            "postgres": {
                "command": None,
                "entrypoint": None,
                "image": "postgres@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b",
                "environment": {
                    "POSTGRES_DB": "seshatops_northstar_disposable",
                    "POSTGRES_PASSWORD": "seshatops-local-only",
                    "POSTGRES_USER": "seshatops",
                },
                "networks": {"local": None},
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
                "volumes": [
                    {
                        "type": "volume",
                        "source": "postgres-data",
                        "target": "/var/lib/postgresql/data",
                        "volume": {},
                    }
                ]
            },
            "redpanda": {
                "entrypoint": None,
                "image": "docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07",
                "command": [
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
                "cap_drop": ["ALL"],
                "security_opt": ["no-new-privileges:true"],
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
                "networks": {"local": None},
                "volumes": [
                    {
                        "type": "volume",
                        "source": "redpanda-data",
                        "target": "/var/lib/redpanda/data",
                        "volume": {},
                    }
                ]
            },
            "redpanda-init": {
                "image": "docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07",
                "command": [
                    "-ec",
                    "rpk topic create seshatops.m1.events --brokers redpanda:9092 --partitions 1 --replicas 1 || "
                    "rpk topic describe seshatops.m1.events --brokers redpanda:9092",
                ],
                "entrypoint": ["/bin/sh"],
                "cap_drop": ["ALL"],
                "security_opt": ["no-new-privileges:true"],
                "restart": "no",
                "depends_on": {
                    "redpanda": {"condition": "service_healthy", "required": True}
                },
                "networks": {"local": None},
            },
            "oidc": {
                "command": None,
                "entrypoint": None,
                "image": "ghcr.io/navikt/mock-oauth2-server@sha256:79f51f412caddb1e2120a5ae10d1f203e134f6e8328f1bc63c444acba33c9086",
                "cap_drop": ["ALL"],
                "security_opt": ["no-new-privileges:true"],
                "read_only": True,
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
                "tmpfs": ["/tmp"],
                "environment": {
                    "JSON_CONFIG_PATH": "/app/config.json",
                    "LOG_LEVEL": "warn",
                    "SERVER_PORT": "9090",
                },
                "networks": {
                    "local": {"aliases": ["oidc.seshatops.localhost"]},
                },
                "ports": [
                    {
                        "host_ip": "127.0.0.1",
                        "mode": "ingress",
                        "protocol": "tcp",
                        "published": "9090",
                        "target": 9090,
                    }
                ],
                "volumes": [
                    {
                        "type": "bind",
                        "source": str(release_demo.ROOT / "docker" / "oidc" / "config.json"),
                        "target": "/app/config.json",
                        "read_only": True,
                        "bind": {},
                    }
                ]
            },
            "runtime": {
                "build": {
                    "context": str(release_demo.ROOT),
                    "dockerfile": "docker/go.Dockerfile",
                },
                "cap_drop": ["ALL"],
                "command": None,
                "entrypoint": None,
                "security_opt": ["no-new-privileges:true"],
                "read_only": True,
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
                "tmpfs": ["/tmp"],
                "networks": {"local": None},
                "environment": {
                    "SESHATOPS_AUTH_ASSIGNMENTS": (
                        "northstar-demo-operator|11111111-1111-4111-8111-111111111111|ROLE-OPS-READER, "
                        "northstar-demo-operator|11111111-1111-4111-8111-111111111111|ROLE-PLATFORM-OPERATOR, "
                        "northstar-demo-operator|SCOPE-RUNTIME|ROLE-RELEASE-OBSERVER"
                    ),
                    "SESHATOPS_LOCAL_STACK": "true",
                    "SESHATOPS_LISTEN_ADDR": ":8080",
                    "SESHATOPS_DATABASE_URL": (
                        "postgres://seshatops:seshatops-local-only@postgres:5432/"
                        "seshatops_northstar_disposable?sslmode=disable"
                    ),
                    "SESHATOPS_BROKER_SEEDS": "redpanda:9092",
                    "SESHATOPS_COOKIE_NAME": "seshatops_session",
                    "SESHATOPS_COOKIE_SECURE": "false",
                    "SESHATOPS_FORECAST_PYTHON": "python3",
                    "SESHATOPS_OIDC_AUDIENCE": "seshatops-local",
                    "SESHATOPS_OIDC_CLIENT_ID": "seshatops-local",
                    "SESHATOPS_OIDC_ISSUER": "http://oidc.seshatops.localhost:9090/default",
                    "SESHATOPS_OIDC_REDIRECT_URL": "http://web.seshatops.localhost:5173/auth/callback",
                }
            },
            "web": {
                "build": {
                    "context": str(release_demo.ROOT),
                    "dockerfile": "docker/web.Dockerfile",
                },
                "command": [
                    "npm",
                    "run",
                    "dev",
                    "--",
                    "--host",
                    "0.0.0.0",
                    "--configLoader",
                    "runner",
                ],
                "cap_drop": ["ALL"],
                "entrypoint": None,
                "security_opt": ["no-new-privileges:true"],
                "read_only": True,
                "depends_on": {
                    "runtime": {"condition": "service_healthy", "required": True}
                },
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
                "tmpfs": ["/tmp"],
                "networks": {
                    "local": {"aliases": ["web.seshatops.localhost"]},
                },
                "environment": {
                    "VITE_API_PROXY_TARGET": "http://runtime:8080",
                    "VITE_CACHE_DIR": "/tmp/seshatops-vite",
                },
                "ports": [
                    {
                        "host_ip": "127.0.0.1",
                        "mode": "ingress",
                        "protocol": "tcp",
                        "published": "5173",
                        "target": 5173,
                    }
                ],
            },
        }
    }


def finish_builder(
    builder: release_demo.ResultBuilder,
    *,
    status: str = "passed",
    failure: dict[str, str] | None = None,
) -> dict:
    if not builder.actions:
        builder.action("test action", "test-command", "2026-01-01T00:00:00.000Z", 1, "passed", 0)
    if not builder.expectations:
        builder.expect("test_expectation", True, "complete", "complete")
    builder.observations["fixture_versions"].setdefault("test", release_demo.FIXTURE_VERSION)
    return builder.finish(
        status=status,
        cleanup={"attempted": True, "status": "passed", "actions": []},
        failure=failure,
        diagnostics=[],
    )


class ScenarioParserTests(unittest.TestCase):
    def test_parser_accepts_all_and_each_named_scenario(self) -> None:
        parser = release_demo.build_parser()

        self.assertEqual(parser.parse_args(["all"]).scenario, "all")
        for scenario in release_demo.SCENARIO_ORDER:
            with self.subTest(scenario=scenario):
                self.assertEqual(parser.parse_args([scenario]).scenario, scenario)

    def test_parser_rejects_an_unknown_scenario(self) -> None:
        with redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit) as caught:
                release_demo.build_parser().parse_args(["unknown-scenario"])

        self.assertNotEqual(caught.exception.code, 0)

    def test_main_rejects_campaign_break_expectation_before_running_commands(self) -> None:
        with redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit) as caught:
                release_demo.main(["all", "--break-expectation", "tenant_denial_telemetry"])

        self.assertEqual(caught.exception.code, 2)


class CommandPropagationTests(unittest.TestCase):
    def test_command_runner_propagates_timeout_as_typed_failure(self) -> None:
        def time_out(args, **kwargs):
            raise subprocess.TimeoutExpired(args, kwargs["timeout"])

        runner = release_demo.CommandRunner(time_out)

        with self.assertRaises(release_demo.CommandTimeout) as caught:
            runner.run(["slow-command"], timeout=0.25)

        self.assertIn("0.2s", str(caught.exception))
        self.assertIn("slow-command", str(caught.exception))

    def test_driver_propagates_nonzero_exit_and_records_failed_action(self) -> None:
        def fail(args, **kwargs):
            return subprocess.CompletedProcess(args, 17, stdout="", stderr="declared failure")

        builder = release_demo.ResultBuilder("normal-flow", RELEASE)
        driver = release_demo.DemoDriver(
            release_demo.CommandRunner(fail),
            builder,
            Path("unused-evidence"),
        )

        with self.assertRaises(release_demo.CommandFailure) as caught:
            driver.command("expected zero", ["false"], timeout=1)

        self.assertIn("exited 17", str(caught.exception))
        self.assertEqual(builder.actions[-1]["status"], "failed")
        self.assertEqual(builder.actions[-1]["exit_code"], 17)

    def test_command_failure_redacts_private_local_target_details(self) -> None:
        context_name = "private-context-name"
        snapshot = "/tmp/seshatops-release-demo-private.yaml"

        def fail(args, **kwargs):
            del kwargs
            detail = (
                f"root={release_demo.ROOT} endpoint=unix:///run/user/1000/docker.sock "
                f"snapshot={snapshot} context={context_name}"
            )
            return subprocess.CompletedProcess(args, 2, stdout="", stderr=detail)

        driver = release_demo.DemoDriver(
            release_demo.CommandRunner(fail),
            release_demo.ResultBuilder("normal-flow", RELEASE),
            Path("unused-evidence"),
        )
        args = ["docker", "context", "inspect", context_name, str(release_demo.ROOT), snapshot, "unix:///run/user/1000/docker.sock"]
        with self.assertRaises(release_demo.CommandFailure) as caught:
            driver.command("private failure", args, timeout=1)

        message = str(caught.exception)
        self.assertNotIn(str(release_demo.ROOT), message)
        self.assertNotIn("/run/user/1000", message)
        self.assertNotIn(context_name, message)
        self.assertNotIn(snapshot, message)

    def test_structured_outcome_reads_go_slog_message_field(self) -> None:
        stderr = json.dumps(
            {
                "time": "2026-08-25T00:00:00Z",
                "level": "INFO",
                "msg": "forecast.command.failed",
                "outcome": "unavailable",
            }
        )

        self.assertTrue(release_demo.structured_outcome(stderr, "unavailable"))
        self.assertFalse(release_demo.structured_outcome(stderr, "timeout"))

    def test_metric_parser_rejects_non_finite_samples(self) -> None:
        for sample in ("NaN", "+Inf", "-Inf"):
            with self.subTest(sample=sample):
                with self.assertRaises(release_demo.InvariantFailure):
                    release_demo.metric_value(f"release_metric {sample}\n", "release_metric")
        with self.assertRaises(release_demo.InvariantFailure):
            release_demo.metric_value(
                "# TYPE release_metric counter\n",
                "release_metric",
                'outcome="complete"',
            )

    def test_poll_metrics_waits_for_counter_advance(self) -> None:
        class SequencedClient:
            def __init__(self):
                self.samples = [5, 6]

            def request(self, path, *, accept):
                self.assert_request(path, accept)
                value = self.samples.pop(0)
                return (
                    200,
                    {},
                    (
                        'seshatops_consumer_processing_outcomes_total'
                        f'{{outcome="processed"}} {value}\n'
                    ).encode(),
                )

            @staticmethod
            def assert_request(path, accept):
                if path != "/metrics" or accept != "text/plain":
                    raise AssertionError((path, accept))

        builder = release_demo.ResultBuilder("duplicate-delivery", RELEASE)
        driver = release_demo.DemoDriver(
            release_demo.CommandRunner(),
            builder,
            Path("unused-evidence"),
        )
        with mock.patch.object(release_demo.time, "sleep", return_value=None):
            observed = driver.poll_metrics(
                SequencedClient(),
                lambda text: release_demo.metric_value(
                    text,
                    "seshatops_consumer_processing_outcomes_total",
                    'outcome="processed"',
                )
                > 5,
                timeout=1,
                description="wait for duplicate consumer telemetry",
            )

        self.assertIn('outcome="processed"} 6', observed)
        self.assertEqual(
            builder.observations["counts"][
                "wait_for_duplicate_consumer_telemetry_poll_attempts"
            ],
            2,
        )

    def test_http_client_disables_ambient_proxy_routing(self) -> None:
        with mock.patch.dict(
            release_demo.os.environ,
            {"HTTP_PROXY": "http://undeclared.example:3128"},
        ):
            client = release_demo.SessionClient()
        handlers = [
            handler
            for handler in client.opener.handlers
            if isinstance(handler, release_demo.ProxyHandler)
        ]
        self.assertEqual(handlers, [])


class SSECaptureTests(unittest.TestCase):
    def test_subscription_is_ready_only_after_post_registration_heartbeat(self) -> None:
        entered_stream = threading.Event()
        emit_heartbeat = threading.Event()
        start_finished = threading.Event()
        errors = []

        class ControlledResponse:
            status = 200

            def __iter__(self):
                entered_stream.set()
                emit_heartbeat.wait(2)
                yield b": heartbeat\n"
                yield b"\n"

            def close(self):
                return None

        class ControlledOpener:
            def open(self, request, timeout):
                del request, timeout
                return ControlledResponse()

        client = release_demo.SessionClient()
        client.opener = ControlledOpener()
        capture = release_demo.SSECapture(client, "/stream", "inventory_projection.updated")

        def start_capture():
            try:
                capture.start()
            except BaseException as error:
                errors.append(error)
            finally:
                start_finished.set()

        starter = threading.Thread(target=start_capture)
        starter.start()
        self.assertTrue(entered_stream.wait(1))
        self.assertFalse(capture.ready.is_set())
        self.assertFalse(start_finished.is_set())
        emit_heartbeat.set()
        self.assertTrue(start_finished.wait(1))
        starter.join(timeout=1)
        self.assertEqual(errors, [])


class ResultSchemaTests(unittest.TestCase):
    def test_success_and_failure_results_validate(self) -> None:
        successful = release_demo.ResultBuilder("normal-flow", RELEASE)
        successful.deterministic_values = {"checksum": "abc"}
        success_result = finish_builder(successful)
        release_demo.validate_result_schema(success_result)
        self.assertEqual(success_result["status"], "passed")
        self.assertIsNone(success_result["failure"])

        failed = release_demo.ResultBuilder("normal-flow", RELEASE)
        failed.deterministic_values = {"checksum": "abc"}
        failure_result = finish_builder(
            failed,
            status="failed",
            failure={"category": "invariant", "message": "expected value was absent"},
        )
        release_demo.validate_result_schema(failure_result)
        self.assertEqual(failure_result["status"], "failed")
        self.assertEqual(failure_result["failure"]["category"], "invariant")

    def test_passing_result_requires_actions_expectations_and_fixture_evidence(self) -> None:
        builder = release_demo.ResultBuilder("normal-flow", RELEASE)
        with self.assertRaises(release_demo.InvariantFailure):
            builder.finish(
                status="passed",
                cleanup={"attempted": True, "status": "passed", "actions": []},
                failure=None,
                diagnostics=[],
            )

    def test_noncomplete_forecast_snapshot_requires_hash_identities(self) -> None:
        builder = release_demo.ResultBuilder("forecast-source-states", RELEASE)
        driver = release_demo.DemoDriver(
            release_demo.CommandRunner(),
            builder,
            Path("unused-evidence"),
        )
        with self.assertRaises(release_demo.InvariantFailure):
            release_demo.expect_noncomplete_snapshot(
                driver,
                "incomplete",
                {
                    "status": "incomplete",
                    "rows": [],
                    "status_reasons": ["malformed retained event"],
                    "snapshot_id": None,
                    "checksum": None,
                },
                "incomplete",
            )

    def test_action_command_is_bounded_and_confirmation_values_are_redacted(self) -> None:
        command = release_demo.safe_command(
            [
                "docker",
                "--host",
                "unix:///run/user/1000/docker.sock",
                "compose",
                "--project-directory",
                str(release_demo.ROOT),
                "--file",
                "/tmp/seshatops-release-demo-private.yaml",
                f"{release_demo.FIXTURE_CONFIRM_ENV}={release_demo.DEMO_CONFIRMATION}",
                f"--forecast-confirm={release_demo.FORECAST_CONFIRMATION}",
                "x" * (release_demo.MAX_FAILURE_TEXT * 2),
            ]
        )
        builder = release_demo.ResultBuilder("normal-flow", RELEASE)
        builder.action("fixture", command, "2026-01-01T00:00:00.000Z", 1, "passed", 0)
        result = finish_builder(builder)

        recorded = result["actions"][0]["command"]
        self.assertLessEqual(len(recorded), release_demo.MAX_FAILURE_TEXT)
        self.assertNotIn(release_demo.DEMO_CONFIRMATION, recorded)
        self.assertNotIn(release_demo.FORECAST_CONFIRMATION, recorded)
        self.assertIn(f"{release_demo.FIXTURE_CONFIRM_ENV}=<confirmation>", recorded)
        self.assertIn("<local-unix-endpoint>", recorded)
        self.assertIn("<repository-root>", recorded)
        self.assertIn("<validated-compose-snapshot>", recorded)
        self.assertNotIn(str(release_demo.ROOT), recorded)
        self.assertNotIn("/run/user/1000", recorded)
        context_command = release_demo.safe_command(
            ["docker", "context", "inspect", "private-context-name"]
        )
        self.assertIn("<selected-local-context>", context_command)
        self.assertNotIn("private-context-name", context_command)

    def test_human_summary_contains_preconditions_and_cleanup_actions(self) -> None:
        builder = release_demo.ResultBuilder("normal-flow", RELEASE)
        result = builder.finish(
            status="failed",
            cleanup={
                "attempted": True,
                "status": "failed",
                "actions": [
                    {
                        "command": "docker compose down --volumes --remove-orphans",
                        "started_at": "2026-01-01T00:00:00.000Z",
                        "duration_ms": 10,
                        "exit_code": 1,
                        "status": "failed",
                    }
                ],
                "error": "cleanup refused",
            },
            failure={"category": "cleanup", "message": "cleanup refused"},
            diagnostics=["normal-flow.runtime-redpanda.log"],
        )

        summary = release_demo.render_human_summary(result)

        self.assertIn("Preconditions:\n- Fresh packaged stack", summary)
        self.assertIn("docker compose down --volumes --remove-orphans", summary)
        self.assertIn("cleanup refused", summary)
        self.assertIn("normal-flow.runtime-redpanda.log", summary)


class CleanupTests(unittest.TestCase):
    class RecordingRunner:
        def __init__(self, cleanup_code: int):
            self.cleanup_code = cleanup_code
            self.calls: list[list[str]] = []

        def run(self, args, *, timeout, input_text=None, env=None):
            del timeout, input_text, env
            self.calls.append(list(args))
            return release_demo.CommandResult(
                args=list(args),
                returncode=self.cleanup_code,
                stdout="",
                stderr="cleanup refused" if self.cleanup_code else "",
                duration_ms=7,
            )

    @staticmethod
    def mark_guarded(
        driver: release_demo.DemoDriver,
        expected_release: dict,
    ) -> None:
        del expected_release
        driver.docker_endpoint = "unix:///var/run/docker.sock"
        driver.compose_snapshot = release_demo.COMPOSE_FILE
        driver.guarded = True

    def test_cleanup_runs_after_scenario_failure(self) -> None:
        runner = self.RecordingRunner(0)

        def fail_scenario(driver):
            del driver
            raise release_demo.InvariantFailure("scenario failed deliberately")

        with tempfile.TemporaryDirectory() as directory:
            with (
                mock.patch.object(release_demo.DemoDriver, "fresh_start", self.mark_guarded),
                mock.patch.object(
                    release_demo.DemoDriver,
                    "verify_release_identity",
                    return_value=None,
                ),
                mock.patch.dict(release_demo.SCENARIOS, {"normal-flow": fail_scenario}),
                mock.patch.object(
                    release_demo,
                    "capture_diagnostics",
                    side_effect=OSError("diagnostic filesystem unavailable"),
                ),
                redirect_stdout(io.StringIO()),
            ):
                result = release_demo.run_one(
                    "normal-flow",
                    release=RELEASE,
                    evidence_dir=Path(directory),
                    runner=runner,
                )

        self.assertEqual(result["status"], "failed")
        self.assertEqual(result["failure"]["category"], "invariant")
        self.assertEqual(result["cleanup"]["status"], "passed")
        self.assertEqual(
            runner.calls,
            [
                release_demo.pinned_compose_command(
                    "unix:///var/run/docker.sock",
                    release_demo.COMPOSE_FILE,
                    "down",
                    "--volumes",
                    "--remove-orphans",
                )
            ],
        )

    def test_cleanup_failure_makes_successful_scenario_fail(self) -> None:
        runner = self.RecordingRunner(9)

        def pass_scenario(driver):
            driver.builder.deterministic_values = {"outcome": "complete"}

        with tempfile.TemporaryDirectory() as directory:
            with (
                mock.patch.object(release_demo.DemoDriver, "fresh_start", self.mark_guarded),
                mock.patch.object(
                    release_demo.DemoDriver,
                    "verify_release_identity",
                    return_value=None,
                ),
                mock.patch.dict(release_demo.SCENARIOS, {"normal-flow": pass_scenario}),
                redirect_stdout(io.StringIO()),
            ):
                result = release_demo.run_one(
                    "normal-flow",
                    release=RELEASE,
                    evidence_dir=Path(directory),
                    runner=runner,
                )

        self.assertEqual(result["status"], "failed")
        self.assertEqual(result["cleanup"]["status"], "failed")
        self.assertIn("cleanup refused", result["failure"]["message"])


class ComposeGuardTests(unittest.TestCase):
    def assert_guard_rejects(self, config=None, **overrides) -> None:
        values = {
            "config": config if config is not None else valid_compose_config(),
            "project_name": release_demo.PROJECT_NAME,
            "compose_file": release_demo.COMPOSE_FILE,
            "confirmation": release_demo.DEMO_CONFIRMATION,
        }
        values.update(overrides)
        with self.assertRaises(release_demo.GuardError):
            release_demo.validate_compose_target(**values)

    def test_guard_accepts_only_declared_target(self) -> None:
        release_demo.validate_compose_target(
            valid_compose_config(),
            project_name=release_demo.PROJECT_NAME,
            compose_file=release_demo.COMPOSE_FILE,
            confirmation=release_demo.DEMO_CONFIRMATION,
        )

    def test_guard_rejects_wrong_project_file_and_confirmation_token(self) -> None:
        self.assert_guard_rejects(project_name="another-project")
        self.assert_guard_rejects(compose_file=release_demo.ROOT / "another-compose.yaml")
        self.assert_guard_rejects(confirmation="wrong-confirmation")

    def test_guard_rejects_wrong_database_broker_or_local_flag(self) -> None:
        cases = {
            "database host": ("SESHATOPS_DATABASE_URL", "postgresql://user:pass@db.example/seshatops_northstar_disposable"),
            "database name": ("SESHATOPS_DATABASE_URL", "postgresql://user:pass@postgres/not_disposable"),
            "database user": ("SESHATOPS_DATABASE_URL", "postgresql://admin:pass@postgres:5432/seshatops_northstar_disposable?sslmode=disable"),
            "broker": ("SESHATOPS_BROKER_SEEDS", "broker.example:9092"),
            "local flag": ("SESHATOPS_LOCAL_STACK", "false"),
        }
        for name, (key, value) in cases.items():
            with self.subTest(name=name):
                config = valid_compose_config()
                config["services"]["runtime"]["environment"][key] = value
                self.assert_guard_rejects(config=config)

    def test_guard_rejects_database_target_overrides(self) -> None:
        for database_url in (
            "postgresql://user:pass@postgres:5433/seshatops_northstar_disposable?sslmode=disable",
            "postgresql://user:pass@postgres:5432/seshatops_northstar_disposable?sslmode=disable&host=203.0.113.10",
            "postgresql://user:pass@postgres:5432/seshatops_northstar_disposable?sslmode=disable&hostaddr=203.0.113.10",
            "postgresql://user:pass@postgres:5432/seshatops_northstar_disposable?sslmode=disable&dbname=production",
            "postgresql://user:pass@postgres:5432/seshatops_northstar_disposable?sslmode=require",
        ):
            with self.subTest(database_url=database_url):
                config = valid_compose_config()
                config["services"]["runtime"]["environment"]["SESHATOPS_DATABASE_URL"] = database_url
                self.assert_guard_rejects(config=config)

    def test_guard_rejects_host_ports_on_protected_services(self) -> None:
        for service in ("postgres", "redpanda", "runtime"):
            with self.subTest(service=service):
                config = valid_compose_config()
                config["services"][service]["ports"] = [{"published": "5432"}]
                self.assert_guard_rejects(config=config)

    def test_guard_rejects_external_http_or_oidc_routes(self) -> None:
        changes = {
            "external web proxy": lambda config: config["services"]["web"]["environment"].update(
                {"VITE_API_PROXY_TARGET": "https://undeclared.example"}
            ),
            "external OIDC issuer": lambda config: config["services"]["runtime"]["environment"].update(
                {"SESHATOPS_OIDC_ISSUER": "https://undeclared.example/default"}
            ),
            "public web listener": lambda config: config["services"]["web"]["ports"][0].update(
                {"host_ip": "0.0.0.0"}
            ),
        }
        for name, change in changes.items():
            with self.subTest(name=name):
                config = valid_compose_config()
                change(config)
                self.assert_guard_rejects(config=config)

    def test_guard_rejects_repackaged_or_privileged_services(self) -> None:
        changes = {
            "replacement postgres image": lambda config: config["services"]["postgres"].update(
                {"image": "postgres:latest"}
            ),
            "replacement runtime context": lambda config: config["services"]["runtime"]["build"].update(
                {"context": "/tmp/undeclared-runtime"}
            ),
            "replacement web Dockerfile": lambda config: config["services"]["web"]["build"].update(
                {"dockerfile": "docker/undeclared.Dockerfile"}
            ),
            "replacement broker command": lambda config: config["services"]["redpanda"].update(
                {"command": ["proxy-to-undeclared-broker"]}
            ),
            "replacement runtime entrypoint": lambda config: config["services"]["runtime"].update(
                {"entrypoint": ["/bin/sh"]}
            ),
            "privileged runtime": lambda config: config["services"]["runtime"].update(
                {"privileged": True}
            ),
            "runtime host capability": lambda config: config["services"]["runtime"].update(
                {"cap_add": ["SYS_ADMIN"]}
            ),
            "runtime secret mount": lambda config: config["services"]["runtime"].update(
                {"secrets": [{"source": "undeclared"}]}
            ),
            "privileged lifecycle hook": lambda config: config["services"]["runtime"].update(
                {"post_start": [{"command": ["/bin/sh"], "privileged": True}]}
            ),
            "replacement healthcheck": lambda config: config["services"]["runtime"][
                "healthcheck"
            ].update({"test": ["CMD", "/bin/sh", "-c", "touch /tmp/guard-bypass"]}),
            "replacement dependency": lambda config: config["services"]["runtime"][
                "depends_on"
            ].update({"postgres": {"condition": "service_started", "required": True}}),
            "undeclared deployment controls": lambda config: config["services"]["runtime"].update(
                {"deploy": {"replicas": 2}}
            ),
        }
        for name, change in changes.items():
            with self.subTest(name=name):
                config = valid_compose_config()
                change(config)
                self.assert_guard_rejects(config=config)

    def test_guard_rejects_shared_external_or_retargeted_volumes(self) -> None:
        changes = {
            "shared postgres volume": lambda config: config["volumes"]["postgres-data"].update(
                {"name": "shared-important-data"}
            ),
            "external broker volume": lambda config: config["volumes"]["redpanda-data"].update(
                {"external": True}
            ),
            "retargeted postgres mount": lambda config: config["services"]["postgres"]["volumes"][0].update(
                {"source": "redpanda-data"}
            ),
            "extra runtime mount": lambda config: config["services"]["runtime"].update(
                {
                    "volumes": [
                        {
                            "type": "bind",
                            "source": "/tmp",
                            "target": "/host-data",
                        }
                    ]
                }
            ),
        }
        for name, change in changes.items():
            with self.subTest(name=name):
                config = valid_compose_config()
                change(config)
                self.assert_guard_rejects(config=config)

    def test_guard_rejects_external_service_resolution(self) -> None:
        changes = {
            "external postgres host": lambda config: config["services"]["runtime"].update(
                {"extra_hosts": ["postgres=203.0.113.10"]}
            ),
            "host network": lambda config: config["services"]["runtime"].update(
                {"network_mode": "host"}
            ),
            "external network": lambda config: config["networks"]["local"].update(
                {"external": True}
            ),
            "second runtime network": lambda config: config["services"]["runtime"].update(
                {"networks": {"local": None, "other": None}}
            ),
        }
        for name, change in changes.items():
            with self.subTest(name=name):
                config = valid_compose_config()
                change(config)
                self.assert_guard_rejects(config=config)


class DockerGuardTests(unittest.TestCase):
    def test_guard_accepts_only_local_unix_endpoint_without_overrides(self) -> None:
        release_demo.validate_docker_environment({})
        release_demo.validate_local_docker_endpoint("unix:///var/run/docker.sock")
        release_demo.validate_local_docker_endpoint("unix:///run/user/1000/docker.sock")

    def test_guard_rejects_remote_or_ambiguous_docker_targets(self) -> None:
        for endpoint in (
            "tcp://127.0.0.1:2375",
            "tcp://docker.example:2376",
            "ssh://operator@docker.example",
            "npipe:////./pipe/docker_engine",
            "unix://relative/docker.sock",
            "",
        ):
            with self.subTest(endpoint=endpoint):
                with self.assertRaises(release_demo.GuardError):
                    release_demo.validate_local_docker_endpoint(endpoint)

        for environment in (
            {"DOCKER_HOST": "unix:///var/run/docker.sock"},
            {"DOCKER_HOST": "tcp://docker.example:2376"},
            {"DOCKER_CONTEXT": "desktop-linux"},
            {"DOCKER_CONTEXT": "remote"},
        ):
            with self.subTest(environment=environment):
                with self.assertRaises(release_demo.GuardError):
                    release_demo.validate_docker_environment(environment)

    def test_compose_snapshot_pins_reviewed_file_contents(self) -> None:
        builder = release_demo.ResultBuilder("normal-flow", RELEASE)
        driver = release_demo.DemoDriver(
            release_demo.CommandRunner(),
            builder,
            Path("unused-evidence"),
        )
        driver.docker_endpoint = "unix:///var/run/docker.sock"
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "compose.yaml"
            source.write_text("name: reviewed\n")
            with mock.patch.object(release_demo, "COMPOSE_FILE", source):
                driver.snapshot_compose_package()
            source.write_text("name: changed\n")

            self.assertEqual(driver.compose_snapshot.read_text(), "name: reviewed\n")
            command = driver.compose_command("config")
            self.assertIn(str(driver.compose_snapshot), command)
            self.assertNotIn(str(source), command)
            self.assertIsNone(driver.remove_compose_snapshot())
            self.assertFalse(driver.compose_snapshot and driver.compose_snapshot.exists())


class DeterminismTests(unittest.TestCase):
    @staticmethod
    def comparable_campaign() -> dict:
        identities = {
            scenario: release_demo.stable_identity({"scenario": scenario})
            for scenario in release_demo.SCENARIO_ORDER
        }
        return {
            "schema_version": release_demo.SCHEMA_VERSION,
            "kind": "campaign_result",
            "release": RELEASE,
            "fixture_version": release_demo.FIXTURE_VERSION,
            "status": "passed",
            "failure": None,
            "scenarios": [
                {
                    "scenario": scenario,
                    "status": "passed",
                    "deterministic_identity": identities[scenario],
                }
                for scenario in release_demo.SCENARIO_ORDER
            ],
            "deterministic_identities": identities,
        }

    def test_deterministic_identity_ignores_runtime_timing(self) -> None:
        first = release_demo.ResultBuilder("normal-flow", RELEASE)
        first.action("same action", "command", "2026-01-01T00:00:00.000Z", 1, "passed")
        first.observations["durations_ms"]["recovery"] = 100
        first.deterministic_values = {"checksum": "abc", "count": 5}
        first_result = finish_builder(first)

        second = release_demo.ResultBuilder("normal-flow", RELEASE)
        second.action("same action", "command", "2027-12-31T23:59:59.000Z", 999_999, "passed")
        second.observations["durations_ms"]["recovery"] = 999_999
        second.deterministic_values = {"count": 5, "checksum": "abc"}
        second_result = finish_builder(second)

        self.assertNotEqual(first_result["actions"], second_result["actions"])
        self.assertNotEqual(first_result["observations"], second_result["observations"])
        self.assertEqual(
            first_result["deterministic_identity"],
            second_result["deterministic_identity"],
        )

    def test_source_digest_detects_dirty_file_content_changes(self) -> None:
        class ListingRunner:
            def run(self, args, *, timeout, input_text=None, env=None):
                del timeout, input_text, env
                self_args = list(args)
                return release_demo.CommandResult(
                    args=self_args,
                    returncode=0,
                    stdout="tracked.txt\0new.txt\0",
                    stderr="",
                    duration_ms=1,
                )

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "tracked.txt").write_text("first")
            (root / "new.txt").write_text("fixture")
            with mock.patch.object(release_demo, "ROOT", root):
                first = release_demo.source_digest(ListingRunner())
                (root / "tracked.txt").write_text("second")
                second = release_demo.source_digest(ListingRunner())

        self.assertNotEqual(first, second)

    def test_source_digest_handles_embedded_nul_without_record_ambiguity(self) -> None:
        class ListingRunner:
            def run(self, args, *, timeout, input_text=None, env=None):
                del args, timeout, input_text, env
                return release_demo.CommandResult(
                    args=[],
                    returncode=0,
                    stdout="binary.dat\0",
                    stderr="",
                    duration_ms=1,
                )

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / "binary.dat"
            path.write_bytes(b"prefix\0path\0mode\0file\0suffix")
            with mock.patch.object(release_demo, "ROOT", root):
                first = release_demo.source_digest(ListingRunner())
                path.write_bytes(b"prefix\0path\0mode\0file\0changed")
                second = release_demo.source_digest(ListingRunner())

        self.assertNotEqual(first, second)

    def test_evidence_inside_repo_must_use_ignored_directory(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            with mock.patch.object(release_demo, "ROOT", root):
                release_demo.validate_evidence_dir(root / ".release-evidence" / "run")
                release_demo.validate_evidence_dir(root.parent / "external-evidence")
                with self.assertRaises(release_demo.DemoError):
                    release_demo.validate_evidence_dir(root / "review-output")

    def test_campaign_comparison_rejects_identity_mismatch(self) -> None:
        current = self.comparable_campaign()
        previous = copy.deepcopy(current)
        different_identity = release_demo.stable_identity(
            {"checksum": "previous"}
        )
        previous["deterministic_identities"]["normal-flow"] = different_identity
        previous["scenarios"][0]["deterministic_identity"] = different_identity

        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "campaign.json"
            path.write_text(json.dumps(previous))
            with self.assertRaises(release_demo.InvariantFailure) as caught:
                release_demo.compare_campaign(path, current)

        self.assertIn("deterministic identities differ", str(caught.exception))

    def test_campaign_comparison_rejects_failed_or_incomplete_prior_run(self) -> None:
        current = self.comparable_campaign()
        for mutate in (
            lambda previous: previous.update(
                {"status": "failed", "failure": {"category": "cleanup", "message": "failed"}}
            ),
            lambda previous: previous["scenarios"].pop(),
            lambda previous: previous["scenarios"][-1].update({"status": "failed"}),
        ):
            with self.subTest(mutate=mutate):
                previous = copy.deepcopy(current)
                mutate(previous)
                with tempfile.TemporaryDirectory() as directory:
                    path = Path(directory) / "campaign.json"
                    path.write_text(json.dumps(previous))
                    with self.assertRaises(release_demo.InvariantFailure):
                        release_demo.compare_campaign(path, current)

    def test_campaign_comparison_records_only_bounded_file_reference(self) -> None:
        current = self.comparable_campaign()
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "campaign.json"
            path.write_text(json.dumps(current))
            comparison = release_demo.compare_campaign(path, current)

        self.assertEqual(comparison["previous"], "<prior-campaign>")

    def test_campaign_comparison_rejects_unbounded_or_nonregular_input(self) -> None:
        current = self.comparable_campaign()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            oversized = root / "oversized.json"
            oversized.write_bytes(b" " * (release_demo.MAX_RESULT_BYTES + 1))
            with self.assertRaises(release_demo.InvariantFailure):
                release_demo.compare_campaign(oversized, current)

            fifo = root / "campaign.fifo"
            release_demo.os.mkfifo(fifo)
            with self.assertRaises(release_demo.InvariantFailure):
                release_demo.compare_campaign(fifo, current)


if __name__ == "__main__":
    unittest.main()
