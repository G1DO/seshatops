"""One-shot, read-only adapter for the Go-owned runtime forecast boundary.

The selected candidate artifact is passed as model context. This process does
not connect to PostgreSQL, receive browser traffic, authorize work, or write
business state; Go validates both the request and response.
"""

from __future__ import annotations

import json
import sys
from typing import Any


CONTRACT_VERSION = 1
PREDICTOR = "candidate"
TARGET = "stockout-within-horizon"
HORIZON_DAYS = 7
STATUS_PREDICTED = "predicted"
STATUS_ABSTAINED = "abstained"
ABSTENTION_UNSUPPORTED_INPUT = "unsupported_feature_input"


def build_response(request: dict[str, Any]) -> dict[str, Any]:
    _validate_request(request)
    response = {
        "contract_version": CONTRACT_VERSION,
        "predictor": PREDICTOR,
        "tenant_id": request["tenant_id"],
        "item_id": request["item_id"],
        "row_id": request["row_id"],
        "observation_date": request["observation_date"],
        "source_cutoff_date": request["observation_date"],
        "target": TARGET,
        "horizon_days": HORIZON_DAYS,
        "feature_definition_version": request["feature_definition_version"],
        "feature_snapshot_id": request["feature_snapshot_id"],
        "feature_snapshot_checksum": request["feature_snapshot_checksum"],
        "model_version": request["model_version"],
        "code_version": request["code_version"],
    }
    artifact = request.get("model_artifact")
    if not isinstance(artifact, dict):
        return _abstained(response)

    for prediction in artifact.get("predictions", []):
        if isinstance(prediction, dict) and prediction.get("row_id") == request["row_id"]:
            if prediction.get("target") != TARGET or prediction.get("horizon_days") != HORIZON_DAYS or prediction.get("source_cutoff_date") != request["observation_date"]:
                return _abstained(response)
            response.update(
                {
                    "status": prediction.get("status"),
                    "stockout_risk": prediction.get("stockout_risk"),
                    "uncertainty": prediction.get("uncertainty"),
                    "abstention_reason": prediction.get("abstention_reason", ""),
                }
            )
            return response
    return _abstained(response)


def _abstained(response: dict[str, Any]) -> dict[str, Any]:
    response.update(
        {
            "status": STATUS_ABSTAINED,
            "stockout_risk": None,
            "uncertainty": None,
            "abstention_reason": ABSTENTION_UNSUPPORTED_INPUT,
        }
    )
    return response


def _validate_request(request: dict[str, Any]) -> None:
    required = (
        "contract_version",
        "predictor",
        "tenant_id",
        "item_id",
        "row_id",
        "observation_date",
        "target",
        "horizon_days",
        "feature_definition_version",
        "feature_snapshot_id",
        "feature_snapshot_checksum",
        "model_version",
        "code_version",
    )
    if not isinstance(request, dict):
        raise ValueError("runtime request fields are required")
    for field in required:
        if field in {"contract_version", "horizon_days"}:
            continue
        if not isinstance(request.get(field), str) or not request[field].strip():
            raise ValueError("runtime request fields are required")
    if request.get("contract_version") != CONTRACT_VERSION or request.get("predictor") != PREDICTOR or request.get("target") != TARGET or request.get("horizon_days") != HORIZON_DAYS:
        raise ValueError("runtime request contract mismatch")
    feature = request.get("feature")
    if not isinstance(feature, dict) or feature.get("row_id") != request["row_id"] or feature.get("tenant_id") != request["tenant_id"] or feature.get("item_id") != request["item_id"] or feature.get("as_of_date") != request["observation_date"] or feature.get("source_cutoff_date") != request["observation_date"]:
        raise ValueError("runtime feature mismatch")


def main() -> int:
    try:
        document = json.load(sys.stdin)
        response = build_response(document)
        json.dump(response, sys.stdout, sort_keys=True, separators=(",", ":"))
        sys.stdout.write("\n")
        return 0
    except (ValueError, TypeError, json.JSONDecodeError) as exc:
        print(json.dumps({"error": str(exc)}, sort_keys=True), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
