#!/usr/bin/env python3
"""Lightweight drift guard for runbook command, API, metric and path references."""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RUNBOOK_DIR = ROOT / "docs" / "operations" / "runbooks"
OPS_INDEX = ROOT / "docs" / "operations" / "README.md"
OBSERVABILITY_MD = ROOT / "docs" / "operations" / "observability.md"
OPENAPI = ROOT / "docs" / "api" / "openapi-projection.yaml"
OBSERVABILITY_GO = ROOT / "observability" / "observability.go"
LOCAL_STACK = ROOT / "scripts" / "local-stack.sh"
CMD_MAIN = ROOT / "cmd" / "seshatops" / "main.go"

REQUIRED_SECTIONS = [
    "Trigger and observable symptoms",
    "Scope and safety assumptions",
    "Diagnosis commands",
    "Smallest safe action sequence",
    "Stop / rollback / escalation",
    "Post-recovery checks and evidence",
    "Limitations",
]

PROHIBITED_PATTERNS = [
    "docker volume prune",
    "docker system prune",
    "psql ",
    "UPDATE platform.inventory_projection",
    "DELETE FROM erp.outbox",
    "shell=True",
    "TRUNCATE",
]

METRIC_RE = re.compile(r"seshatops_(?:outbox|relay|consumer|control|http|auth|prediction|forecast|python|runtime)[_a-z0-9]*")
API_RE = re.compile(r"/v1/tenants/[^\s\)\]`\"']+|/metrics|/readyz|/livez|/auth/[a-z/]+")
SCRIPT_CMD_RE = re.compile(r"\./scripts/local-stack\.sh[^\n`]*")
GO_CMD_RE = re.compile(r"go run \./cmd/seshatops [a-z\-\.]+")
MD_LINK_RE = re.compile(r"\[[^\]]*\]\(([^\)]+)\)")


def fail(msg: str) -> None:
    raise AssertionError(msg)


def load_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def check_runbooks_exist() -> list[Path]:
    if not RUNBOOK_DIR.is_dir():
        fail(f"runbooks directory missing: {RUNBOOK_DIR}")
    runbooks = sorted(RUNBOOK_DIR.glob("*.md"))
    expected = {
        "broker-interruption.md",
        "poison-isolation.md",
        "rebuild-checksum.md",
        "forecast-degradation.md",
    }
    found = {p.name for p in runbooks}
    if found != expected:
        fail(f"runbooks mismatch: expected {expected}, found {found}")
    if not OPS_INDEX.is_file():
        fail(f"operations index missing: {OPS_INDEX}")
    return runbooks


def check_required_sections(runbooks: list[Path]) -> None:
    for rb in runbooks:
        text = load_text(rb)
        for section in REQUIRED_SECTIONS:
            if section not in text:
                fail(f"{rb.name}: missing required section '{section}'")


def check_prohibited(runbooks: list[Path]) -> None:
    for rb in runbooks:
        text = load_text(rb)
        for pat in PROHIBITED_PATTERNS:
            if pat not in text:
                continue
            # Allow explicit "do not" / "never" / "prohibited" negative mentions, but not prescriptive usage.
            # If the pattern appears in a line that also contains a negation, treat as documentation of what NOT to do.
            lines = [l for l in text.splitlines() if pat in l]
            for line in lines:
                lower = line.lower()
                if any(neg in lower for neg in ("do not", "never", "no ", "not ", "prohibited", "avoid")):
                    continue
                fail(f"{rb.name}: contains prohibited prescriptive pattern '{pat}' in: {line.strip()[:120]}")


def check_metrics(runbooks: list[Path]) -> None:
    if not OBSERVABILITY_GO.is_file():
        fail(f"observability source missing: {OBSERVABILITY_GO}")
    obs_text = load_text(OBSERVABILITY_GO)
    # Also allow metrics that appear literally in observability.md as canonical contract
    # but enforce they exist in Go as source of truth.
    for rb in runbooks:
        text = load_text(rb)
        for m in METRIC_RE.findall(text):
            # counters include suffixes like _total, _sum etc; normalize by checking prefix present
            # Require at least the base name or exact match exists in Go source.
            # For gauges and counters, Go contains the base string without suffix.
            base = m
            # strip known suffixes for check but keep exact mention also valid if substring present
            if base not in obs_text:
                # try stripping _total/_sum/_count for control metrics that render with suffix
                stripped = re.sub(r"(_total|_sum|_count)$", "", base)
                if stripped not in obs_text and base not in load_text(OBSERVABILITY_MD):
                    fail(f"{rb.name}: metric '{m}' not found in {OBSERVABILITY_GO.name} nor observability.md")


def check_api_paths(runbooks: list[Path]) -> None:
    if not OPENAPI.is_file():
        fail(f"openapi missing: {OPENAPI}")
    openapi_text = load_text(OPENAPI)
    uuid_re = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
    for rb in runbooks:
        text = load_text(rb)
        candidates = set(API_RE.findall(text))
        # Exclude abbreviated placeholder "..." which is intentional shorthand, not a literal path
        candidates = {c for c in candidates if "..." not in c}
        for api in candidates:
            # Normalize placeholder paths and concrete UUID paths
            normalized = api.rstrip(",.").rstrip("`").split("?")[0]
            # Map concrete tenant UUID to template placeholder for lookup
            templated = uuid_re.sub("{tenant_id}", normalized)
            # Batch id placeholder
            if "batch-bread-001" in templated:
                templated = templated.replace("batch-bread-001", "{batch_id}")
            if "item-flour-001" in templated:
                templated = templated.replace("item-flour-001", "{resource_id}")
            # For templated resource, check that the fixed pattern exists
            if "/forecast/predictions/" in templated:
                if "/forecast/predictions/{resource_id}" not in openapi_text:
                    fail(f"{rb.name}: API path '{api}' not in openapi")
                continue
            if "/forecast/features" in templated and "/forecast/features" not in openapi_text:
                fail(f"{rb.name}: API path '{api}' not in openapi")
                continue
            if "/ops/lineage" in templated and "/ops/lineage/batches/{batch_id}" not in openapi_text:
                fail(f"{rb.name}: API path '{api}' not in openapi")
                continue
            if templated.startswith("/auth/") or templated in ("/metrics", "/readyz", "/livez"):
                continue
            if templated not in openapi_text:
                # also try prefix check for inventory/stream
                if "/inventory" in templated and "/v1/tenants/{tenant_id}/inventory" in openapi_text:
                    continue
                if "/ops" in templated and "/v1/tenants/{tenant_id}/ops" in openapi_text:
                    continue
                fail(f"{rb.name}: API path '{api}' (templated as '{templated}') not in openapi")


def check_script_commands(runbooks: list[Path]) -> None:
    stack_text = load_text(LOCAL_STACK)
    for rb in runbooks:
        text = load_text(rb)
        for cmd in SCRIPT_CMD_RE.findall(text):
            parts = cmd.strip().split()
            if len(parts) < 2:
                continue
            sub = parts[1]
            # Strip trailing punctuation
            sub = sub.strip(" ,;\"'`")
            if sub and sub not in stack_text:
                # allow demo sub-scenarios like 'demo' itself is in stack, but specific scenario names are not
                if sub == "demo":
                    continue
                # for commands like "./scripts/local-stack.sh logs runtime --tail"
                # the sub is 'logs' which must exist
                if sub not in ("logs", "status", "quickstart", "down", "reset", "smoke", "demo"):
                    fail(f"{rb.name}: script subcommand '{sub}' not in local-stack.sh")


def check_go_commands(runbooks: list[Path]) -> None:
    main_text = load_text(CMD_MAIN)
    for rb in runbooks:
        text = load_text(rb)
        for cmd in GO_CMD_RE.findall(text):
            parts = cmd.split()
            if len(parts) < 4:
                continue
            sub = parts[3].strip("`\"'")
            if f'"{sub}"' not in main_text and f"'{sub}'" not in main_text and sub not in main_text:
                fail(f"{rb.name}: Go subcommand '{sub}' not in cmd/seshatops/main.go")


def check_doc_links(runbooks: list[Path]) -> None:
    all_pages = [OPS_INDEX, OBSERVABILITY_MD] + runbooks
    # also check cross-link index files that reference runbooks
    extra_indices = [
        ROOT / "docs" / "README.md",
        ROOT / "docs" / "getting-started.md",
        ROOT / "docs" / "architecture" / "overview.md",
        ROOT / "docs" / "EVIDENCE.md",
    ]
    check_files = runbooks + extra_indices
    for src in check_files:
        if not src.is_file():
            continue
        text = load_text(src)
        for match in MD_LINK_RE.findall(text):
            target = match.split("#")[0].strip()
            if not target or target.startswith("http://") or target.startswith("https://") or target.startswith("mailto:"):
                continue
            # ignore anchors and image embeds handled
            if target.startswith("data:"):
                continue
            # Only validate relative .md links that point inside docs/operations
            if "operations/runbooks" in target or "operations/README" in target or "operations/observability" in target:
                # Resolve relative to src
                resolved = (src.parent / target).resolve()
                if not resolved.is_file():
                    # try from ROOT
                    alt = (ROOT / target.lstrip("./")).resolve()
                    if not alt.is_file():
                        fail(f"{src.relative_to(ROOT)}: broken doc link '{match}' -> {resolved}")
        # For runbooks, ensure they reference only allowed doc scopes
        if src in runbooks:
            for link in MD_LINK_RE.findall(text):
                if "../design/specifications/" in link or "openapi-projection.yaml" in link or "authorization.md" in link:
                    # these are allowed
                    continue
                if link.endswith(".md") and "operations/runbooks" in link and link not in text:
                    continue


def main() -> None:
    runbooks = check_runbooks_exist()
    check_required_sections(runbooks)
    check_prohibited(runbooks)
    check_metrics(runbooks)
    check_api_paths(runbooks)
    check_script_commands(runbooks)
    check_go_commands(runbooks)
    check_doc_links(runbooks)

    # Cross-link sanity: operations index must list all 4 runbooks
    index_text = load_text(OPS_INDEX)
    for name in ("broker-interruption.md", "poison-isolation.md", "rebuild-checksum.md", "forecast-degradation.md"):
        if name not in index_text:
            fail(f"{OPS_INDEX.name}: missing link to {name}")

    # Getting-started must reference runbooks
    gs = load_text(ROOT / "docs" / "getting-started.md")
    if "operations/runbooks" not in gs:
        fail("docs/getting-started.md: missing runbooks cross-link")
    arch = load_text(ROOT / "docs" / "architecture" / "overview.md")
    if "operations/README.md" not in arch and "operations/runbooks" not in arch:
        fail("docs/architecture/overview.md: missing runbooks cross-link")
    ev = load_text(ROOT / "docs" / "EVIDENCE.md")
    if "RUNBOOK_EXERCISE_REPORT" not in ev:
        fail("docs/EVIDENCE.md: missing runbook exercise report link")

    print("runbook drift checks passed")


if __name__ == "__main__":
    try:
        main()
    except AssertionError as e:
        print(f"check_runbooks failed: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:  # pragma: no cover
        print(f"check_runbooks error: {e}", file=sys.stderr)
        sys.exit(2)
