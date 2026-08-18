import copy
import json
import tempfile
import unittest
from datetime import date, timedelta
from pathlib import Path

from stockout_candidate import (
    ABSTENTION_INSUFFICIENT_SUPPORT,
    CandidateInputError,
    STATUS_ABSTAINED,
    STATUS_PREDICTED,
    build_artifact,
    main,
)


class StockoutCandidateTests(unittest.TestCase):
    def test_predictions_are_deterministic_and_fit_only_train(self) -> None:
        document = make_document()
        first = build_artifact(document).as_dict()

        changed = copy.deepcopy(document)
        for example in changed["dataset"]["examples"]:
            if example["split"] != "train":
                example["label"] = 1 - example["label"]
        second = build_artifact(changed).as_dict()

        self.assertEqual(first, second)

        shuffled = copy.deepcopy(document)
        shuffled["dataset"]["examples"].reverse()
        shuffled["features"]["rows"].reverse()
        self.assertEqual(first, build_artifact(shuffled).as_dict())

    def test_uncertainty_and_insufficient_support_are_explicit(self) -> None:
        artifact = build_artifact(make_document()).as_dict()
        by_id = {prediction["row_id"]: prediction for prediction in artifact["predictions"]}

        predicted = by_id["row-070"]
        self.assertEqual(predicted["status"], STATUS_PREDICTED)
        self.assertIsNotNone(predicted["stockout_risk"])
        uncertainty = predicted["uncertainty"]
        self.assertEqual(uncertainty["method"], "wilson-95")
        self.assertEqual(uncertainty["sample_count"], 5)
        self.assertLessEqual(uncertainty["lower"], predicted["stockout_risk"])
        self.assertGreaterEqual(uncertainty["upper"], predicted["stockout_risk"])

        abstained = by_id["row-084"]
        self.assertEqual(abstained["status"], STATUS_ABSTAINED)
        self.assertIsNone(abstained["stockout_risk"])
        self.assertIsNone(abstained["uncertainty"])
        self.assertEqual(abstained["abstention_reason"], ABSTENTION_INSUFFICIENT_SUPPORT)

    def test_stale_and_malformed_inputs_fail_closed(self) -> None:
        stale = make_document()
        stale["features"]["status"] = "stale"
        with self.assertRaises(CandidateInputError):
            build_artifact(stale)

        malformed = make_document()
        malformed["features"]["rows"][0]["quantity_on_hand"] = -1
        with self.assertRaises(CandidateInputError):
            build_artifact(malformed)

    def test_cli_writes_only_a_prediction_artifact(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            input_path = root / "input.json"
            output_path = root / "artifact.json"
            input_path.write_text(json.dumps(make_document()), encoding="utf-8")

            self.assertEqual(main(["--input", str(input_path), "--output", str(output_path)]), 0)
            artifact = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertEqual(artifact["model_version"], "m4-onhand-bucket-rate-v1")
            self.assertNotIn("label", json.dumps(artifact))


def make_document() -> dict:
    tenant = "11111111-1111-4111-8111-111111111111"
    start = date(2026, 1, 5)
    examples = []
    rows = []
    for index in range(100):
        as_of = (start + timedelta(days=index)).isoformat()
        if index < 5:
            quantity = 8
            label = [1, 0, 1, 0, 0][index]
        elif index < 70:
            quantity = 0
            label = 0
        elif index < 84:
            quantity = 8
            label = index % 2
        else:
            quantity = 3
            label = (index + 1) % 2
        split = "train" if index < 70 else "validation" if index < 84 else "test"
        row_id = f"row-{index:03d}"
        example = {
            "row_id": row_id,
            "tenant_id": tenant,
            "item_id": "item-flour-001",
            "as_of_date": as_of,
            "label": label,
            "split": split,
            "source_cutoff_date": as_of,
            "history_hash": f"history-{index:03d}",
            "protocol_id": "m4-stockout-eval-v1",
        }
        examples.append(example)
        rows.append(
            {
                "row_id": row_id,
                "tenant_id": tenant,
                "item_id": "item-flour-001",
                "as_of_date": as_of,
                "source_cutoff_date": as_of,
                "split": split,
                "quantity_on_hand": quantity,
                "history_hash": example["history_hash"],
            }
        )

    return {
        "dataset_checksum": "dataset-checksum",
        "dataset": {
            "protocol_id": "m4-stockout-eval-v1",
            "tenant_id": tenant,
            "examples": examples,
        },
        "features": {
            "contract_version": 1,
            "status": "complete",
            "tenant_id": tenant,
            "dataset_version": "m4-stockout-eval-v1",
            "feature_definition_version": "m4-raw-onhand-v1",
            "snapshot_id": "snapshot-id",
            "checksum": "snapshot-checksum",
            "rows": rows,
        },
    }


if __name__ == "__main__":
    unittest.main()
