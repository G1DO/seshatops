"""Offline, read-only stockout-risk candidate artifact producer.

The producer consumes a serialized Go-owned dataset and complete feature
snapshot. It never connects to PostgreSQL, calls HTTP, or makes a workflow
decision. Go validates the emitted artifact and owns frozen evaluation and
promotion eligibility.
"""

from __future__ import annotations

import argparse
import json
import math
import sys
from dataclasses import asdict, dataclass
from datetime import date
from typing import Any


ARTIFACT_VERSION = 1
MODEL_VERSION = "m4-onhand-bucket-rate-v1"
CODE_VERSION = "m4-python-stockout-candidate-v1"
PROTOCOL_ID = "m4-stockout-eval-v1"
FEATURE_DEFINITION_VERSION = "m4-raw-onhand-v1"
TARGET = "stockout-within-horizon"
HORIZON_DAYS = 7
TRAINING_SPLIT = "train"
TUNING_SPLIT = "validation"
UNCERTAINTY_METHOD = "wilson-95"
MINIMUM_SUPPORT = 5
WILSON_Z = 1.96

STATUS_PREDICTED = "predicted"
STATUS_ABSTAINED = "abstained"
ABSTENTION_INSUFFICIENT_SUPPORT = "insufficient_training_support"
ABSTENTION_UNSUPPORTED_INPUT = "unsupported_feature_input"


class CandidateInputError(ValueError):
    """Raised when the candidate cannot safely consume its declared input."""


@dataclass(frozen=True)
class Uncertainty:
    method: str
    lower: float
    upper: float
    sample_count: int


@dataclass(frozen=True)
class CandidatePrediction:
    row_id: str
    target: str
    horizon_days: int
    source_cutoff_date: str
    status: str
    stockout_risk: float | None
    uncertainty: Uncertainty | None
    abstention_reason: str | None

    def as_dict(self) -> dict[str, Any]:
        return asdict(self)


@dataclass(frozen=True)
class CandidateArtifact:
    artifact_version: int
    model_version: str
    code_version: str
    evaluation_protocol_version: str
    dataset_version: str
    dataset_checksum: str
    feature_definition_version: str
    feature_snapshot_id: str
    feature_snapshot_checksum: str
    target: str
    horizon_days: int
    training_split: str
    tuning_split: str
    predictions: list[CandidatePrediction]

    def as_dict(self) -> dict[str, Any]:
        result = asdict(self)
        result["predictions"] = [prediction.as_dict() for prediction in self.predictions]
        return result


def build_artifact(document: dict[str, Any]) -> CandidateArtifact:
    dataset, features, dataset_checksum = _validate_input(document)
    examples = dataset["examples"]
    feature_rows = {row["row_id"]: row for row in features["rows"]}
    counts = _fit_train_only(examples, feature_rows)

    predictions: list[CandidatePrediction] = []
    for example in sorted(examples, key=_example_sort_key):
        row = feature_rows[example["row_id"]]
        bucket = _bucket_for(row["quantity_on_hand"])
        if bucket is None:
            predictions.append(
                CandidatePrediction(
                    row_id=example["row_id"],
                    target=TARGET,
                    horizon_days=HORIZON_DAYS,
                    source_cutoff_date=example["as_of_date"],
                    status=STATUS_ABSTAINED,
                    stockout_risk=None,
                    uncertainty=None,
                    abstention_reason=ABSTENTION_UNSUPPORTED_INPUT,
                )
            )
            continue

        sample_count, positive_count = counts.get(bucket, (0, 0))
        if sample_count < MINIMUM_SUPPORT:
            predictions.append(
                CandidatePrediction(
                    row_id=example["row_id"],
                    target=TARGET,
                    horizon_days=HORIZON_DAYS,
                    source_cutoff_date=example["as_of_date"],
                    status=STATUS_ABSTAINED,
                    stockout_risk=None,
                    uncertainty=None,
                    abstention_reason=ABSTENTION_INSUFFICIENT_SUPPORT,
                )
            )
            continue

        risk = (positive_count + 1) / (sample_count + 2)
        lower, upper = _wilson_interval(positive_count, sample_count)
        # Laplace smoothing can move the point estimate slightly outside the
        # raw Wilson interval at the edges. The interval remains conservative
        # by explicitly containing the emitted point estimate.
        lower = min(lower, risk)
        upper = max(upper, risk)
        predictions.append(
            CandidatePrediction(
                row_id=example["row_id"],
                target=TARGET,
                horizon_days=HORIZON_DAYS,
                source_cutoff_date=example["as_of_date"],
                status=STATUS_PREDICTED,
                stockout_risk=risk,
                uncertainty=Uncertainty(
                    method=UNCERTAINTY_METHOD,
                    lower=lower,
                    upper=upper,
                    sample_count=sample_count,
                ),
                abstention_reason=None,
            )
        )

    return CandidateArtifact(
        artifact_version=ARTIFACT_VERSION,
        model_version=MODEL_VERSION,
        code_version=CODE_VERSION,
        evaluation_protocol_version=PROTOCOL_ID,
        dataset_version=dataset["protocol_id"],
        dataset_checksum=dataset_checksum,
        feature_definition_version=features["feature_definition_version"],
        feature_snapshot_id=features["snapshot_id"],
        feature_snapshot_checksum=features["checksum"],
        target=TARGET,
        horizon_days=HORIZON_DAYS,
        training_split=TRAINING_SPLIT,
        tuning_split=TUNING_SPLIT,
        predictions=predictions,
    )


def _validate_input(document: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any], str]:
    if not isinstance(document, dict):
        raise CandidateInputError("input must be an object")
    dataset = _object(document.get("dataset"), "dataset")
    features = _object(document.get("features"), "features")
    dataset_checksum = _nonempty_string(document.get("dataset_checksum"), "dataset_checksum")

    if dataset.get("protocol_id") != PROTOCOL_ID:
        raise CandidateInputError("dataset protocol must be m4-stockout-eval-v1")
    tenant_id = _nonempty_string(dataset.get("tenant_id"), "dataset.tenant_id").lower()
    examples = dataset.get("examples")
    if not isinstance(examples, list) or not examples:
        raise CandidateInputError("dataset.examples must be non-empty")

    seen_rows: set[str] = set()
    for example in examples:
        if not isinstance(example, dict):
            raise CandidateInputError("dataset example must be an object")
        row_id = _nonempty_string(example.get("row_id"), "example.row_id")
        if row_id in seen_rows:
            raise CandidateInputError(f"duplicate example row_id {row_id}")
        seen_rows.add(row_id)
        example_tenant = _nonempty_string(example.get("tenant_id"), f"example.tenant_id for {row_id}").lower()
        if example_tenant != tenant_id:
            raise CandidateInputError(f"example tenant mismatch for {row_id}")
        _nonempty_string(example.get("item_id"), f"example.item_id for {row_id}")
        _date_string(example.get("as_of_date"), f"example.as_of_date for {row_id}")
        if example.get("source_cutoff_date") != example.get("as_of_date"):
            raise CandidateInputError(f"example cutoff mismatch for {row_id}")
        if example.get("label") not in (0, 1):
            raise CandidateInputError(f"example label for {row_id} must be 0 or 1")
        if example.get("protocol_id") != PROTOCOL_ID:
            raise CandidateInputError(f"example protocol mismatch for {row_id}")
        if example.get("split") not in ("train", "validation", "test"):
            raise CandidateInputError(f"unsupported split for {row_id}")

    _validate_frozen_splits(examples)

    if features.get("contract_version") != 1:
        raise CandidateInputError("unsupported feature contract version")
    if features.get("status") != "complete":
        raise CandidateInputError(f"feature snapshot status is {features.get('status')!r}")
    feature_tenant = _nonempty_string(features.get("tenant_id"), "features.tenant_id").lower()
    if feature_tenant != tenant_id:
        raise CandidateInputError("feature tenant does not match dataset tenant")
    if features.get("dataset_version") != PROTOCOL_ID:
        raise CandidateInputError("feature dataset version mismatch")
    if features.get("feature_definition_version") != FEATURE_DEFINITION_VERSION:
        raise CandidateInputError("feature definition version mismatch")
    _nonempty_string(features.get("snapshot_id"), "features.snapshot_id")
    _nonempty_string(features.get("checksum"), "features.checksum")
    rows = features.get("rows")
    if not isinstance(rows, list) or not rows:
        raise CandidateInputError("features.rows must be non-empty")

    example_by_id = {example["row_id"]: example for example in examples}
    feature_by_id: dict[str, dict[str, Any]] = {}
    for row in rows:
        if not isinstance(row, dict):
            raise CandidateInputError("feature row must be an object")
        row_id = _nonempty_string(row.get("row_id"), "feature.row_id")
        if row_id in feature_by_id:
            raise CandidateInputError(f"duplicate feature row_id {row_id}")
        example = example_by_id.get(row_id)
        if example is None:
            raise CandidateInputError(f"extra feature row_id {row_id}")
        row_tenant = _nonempty_string(row.get("tenant_id"), f"feature.tenant_id for {row_id}").lower()
        if row_tenant != tenant_id:
            raise CandidateInputError(f"feature tenant mismatch for {row_id}")
        if row.get("item_id") != example.get("item_id") or row.get("as_of_date") != example.get("as_of_date"):
            raise CandidateInputError(f"feature identity mismatch for {row_id}")
        if row.get("source_cutoff_date") != example.get("as_of_date"):
            raise CandidateInputError(f"feature cutoff mismatch for {row_id}")
        if row.get("split") != example.get("split") or row.get("history_hash") != example.get("history_hash"):
            raise CandidateInputError(f"feature lineage mismatch for {row_id}")
        quantity = row.get("quantity_on_hand")
        if isinstance(quantity, bool) or not isinstance(quantity, int) or quantity < 0:
            raise CandidateInputError(f"feature quantity is unsupported for {row_id}")
        feature_by_id[row_id] = row

    if set(feature_by_id) != set(example_by_id):
        raise CandidateInputError("feature rows do not exactly match dataset rows")
    features["rows"] = list(feature_by_id.values())
    dataset["tenant_id"] = tenant_id
    return dataset, features, dataset_checksum


def _validate_frozen_splits(examples: list[dict[str, Any]]) -> None:
    dates = sorted({example["as_of_date"] for example in examples})
    prior_rank = 0
    for index, as_of_date in enumerate(dates, start=1):
        if index <= 70:
            expected = "train"
            rank = 1
        elif index <= 84:
            expected = "validation"
            rank = 2
        elif index <= 105:
            expected = "test"
            rank = 3
        else:
            raise CandidateInputError(f"date {as_of_date} is outside frozen labeled splits")
        actual = {example["split"] for example in examples if example["as_of_date"] == as_of_date}
        if actual != {expected}:
            raise CandidateInputError(f"frozen split mismatch for {as_of_date}")
        if rank < prior_rank:
            raise CandidateInputError(f"non-chronological split at {as_of_date}")
        prior_rank = rank


def _fit_train_only(examples: list[dict[str, Any]], feature_rows: dict[str, dict[str, Any]]) -> dict[str, tuple[int, int]]:
    counts: dict[str, list[int]] = {}
    for example in examples:
        if example["split"] != TRAINING_SPLIT:
            continue
        bucket = _bucket_for(feature_rows[example["row_id"]]["quantity_on_hand"])
        if bucket is None:
            raise CandidateInputError(f"unsupported training feature for {example['row_id']}")
        count = counts.setdefault(bucket, [0, 0])
        count[0] += 1
        count[1] += example["label"]
    return {bucket: (values[0], values[1]) for bucket, values in counts.items()}


def _bucket_for(quantity: int) -> str | None:
    if isinstance(quantity, bool) or not isinstance(quantity, int) or quantity < 0:
        return None
    if quantity == 0:
        return "zero"
    if quantity <= 2:
        return "low"
    if quantity <= 7:
        return "medium"
    return "high"


def _wilson_interval(positive_count: int, sample_count: int) -> tuple[float, float]:
    proportion = positive_count / sample_count
    denominator = 1 + WILSON_Z**2 / sample_count
    center = (proportion + WILSON_Z**2 / (2 * sample_count)) / denominator
    margin = (
        WILSON_Z
        * math.sqrt(
            proportion * (1 - proportion) / sample_count
            + WILSON_Z**2 / (4 * sample_count**2)
        )
        / denominator
    )
    return max(0.0, center - margin), min(1.0, center + margin)


def _example_sort_key(example: dict[str, Any]) -> tuple[str, str, str, str]:
    return (example["tenant_id"], example["item_id"], example["as_of_date"], example["row_id"])


def _object(value: Any, field: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise CandidateInputError(f"{field} must be an object")
    return value


def _nonempty_string(value: Any, field: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise CandidateInputError(f"{field} must be a non-empty string")
    return value.strip()


def _date_string(value: Any, field: str) -> str:
    text = _nonempty_string(value, field)
    try:
        parsed = date.fromisoformat(text)
    except ValueError as exc:
        raise CandidateInputError(f"{field} must be YYYY-MM-DD") from exc
    if parsed.isoformat() != text:
        raise CandidateInputError(f"{field} must be YYYY-MM-DD")
    return text


def _load_json(path: str) -> dict[str, Any]:
    with open(path, "r", encoding="utf-8") as handle:
        value = json.load(handle)
    if not isinstance(value, dict):
        raise CandidateInputError("input JSON must be an object")
    return value


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="CandidateInput JSON path")
    parser.add_argument("--output", help="Output artifact path; stdout when omitted")
    args = parser.parse_args(argv)
    try:
        artifact = build_artifact(_load_json(args.input))
        payload = json.dumps(artifact.as_dict(), sort_keys=True, separators=(",", ":")) + "\n"
        if args.output:
            with open(args.output, "w", encoding="utf-8") as handle:
                handle.write(payload)
        else:
            sys.stdout.write(payload)
        return 0
    except (CandidateInputError, OSError, json.JSONDecodeError) as exc:
        print(json.dumps({"error": str(exc)}, sort_keys=True), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
