#!/usr/bin/env python3
"""Render and inspect the bounded local Compose package without starting it."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path

import release_demo


ROOT = Path(__file__).resolve().parents[1]
COMPOSE = ["docker", "compose", "--project-name", "seshatops-local", "--file", str(ROOT / "compose.yaml")]


def fail(message):
    raise AssertionError(message)


def main():
    rendered = subprocess.run(
        COMPOSE + ["config", "--format", "json"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    config = json.loads(rendered.stdout)
    release_demo.validate_compose_target(
        config,
        project_name=release_demo.PROJECT_NAME,
        compose_file=ROOT / "compose.yaml",
        confirmation=release_demo.DEMO_CONFIRMATION,
    )
    services = config["services"]
    expected = {"postgres", "redpanda", "redpanda-init", "oidc", "runtime", "web"}
    if set(services) != expected:
        fail(f"services={set(services)!r}")

    for service in ("postgres", "redpanda", "redpanda-init", "oidc"):
        image = services[service].get("image", "")
        if "@sha256:" not in image:
            fail(f"{service} image is not digest-pinned")

    if services["postgres"].get("ports") or services["redpanda"].get("ports") or services["runtime"].get("ports"):
        fail("data, broker, or Go runtime is exposed to the host")
    web_ports = services["web"].get("ports")
    if (
        len(web_ports or []) != 1
        or web_ports[0]["target"] != 5173
        or str(web_ports[0]["published"]) != "5173"
        or web_ports[0].get("host_ip") != "127.0.0.1"
    ):
        fail(f"unexpected web ports={services['web'].get('ports')!r}")
    oidc_ports = services["oidc"].get("ports")
    if (
        len(oidc_ports or []) != 1
        or oidc_ports[0]["target"] != 9090
        or str(oidc_ports[0]["published"]) != "9090"
        or oidc_ports[0].get("host_ip") != "127.0.0.1"
    ):
        fail(f"unexpected OIDC ports={services['oidc'].get('ports')!r}")

    depends = services["runtime"]["depends_on"]
    for dependency in ("postgres", "oidc"):
        if depends[dependency]["condition"] != "service_healthy":
            fail(f"runtime does not wait for healthy {dependency}")
    if depends["redpanda-init"]["condition"] != "service_completed_successfully":
        fail("runtime does not wait for the topic init gate")
    if services["web"]["depends_on"]["runtime"]["condition"] != "service_healthy":
        fail("web does not wait for Go readiness")

    if services["web"]["environment"]["VITE_API_PROXY_TARGET"] != "http://runtime:8080":
        fail("web proxy does not target the Go runtime")
    if services["runtime"].get("read_only") is not True or services["web"].get("read_only") is not True:
        fail("application containers are not read-only")

    oidc = json.loads((ROOT / "docker/oidc/config.json").read_text())
    if oidc.get("interactiveLogin") is not True:
        fail("OIDC interactive login is disabled")
    mappings = oidc["tokenCallbacks"][0]["requestMappings"]
    if mappings[0]["match"] != "northstar-demo-operator" or mappings[1]["match"] != ".*":
        fail("OIDC demo mappings are not bounded")

    stack_script = (ROOT / "scripts/local-stack.sh").read_text()
    if "I_UNDERSTAND_DISPOSABLE_LOCAL_RESET" not in stack_script:
        fail("reset confirmation is missing")
    if "docker volume prune" in stack_script or "docker system prune" in stack_script:
        fail("reset uses a broad Docker prune")
    if "down --volumes --remove-orphans" not in stack_script:
        fail("reset does not remove only the Compose environment")
    if 'demo) demo "$@"' not in stack_script or "scripts/release_demo.py" not in stack_script:
        fail("release demonstration command is not wired through the local stack")
    if "sys.version_info < (3, 10)" not in stack_script:
        fail("release demonstration host Python version guard is missing")

    demo_script = (ROOT / "scripts/release_demo.py").read_text()
    for marker in (
        "seshatops-local",
        "I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO",
        "seshatops_northstar_disposable",
        "redpanda:9092",
        "down\", \"--volumes\", \"--remove-orphans",
    ):
        if marker not in demo_script:
            fail(f"release demonstration guard marker is missing: {marker}")
    if "shell=True" in demo_script:
        fail("release demonstration commands may not use a shell")

    dockerfile = (ROOT / "docker/go.Dockerfile").read_text()
    if "scripts/demo-forecast-timeout.py" not in dockerfile:
        fail("packaged Python timeout fixture is missing")

    print("local stack configuration tests passed")


if __name__ == "__main__":
    main()
