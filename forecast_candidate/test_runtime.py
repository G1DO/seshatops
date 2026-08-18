import unittest

import runtime


class RuntimeAdapterTest(unittest.TestCase):
    def test_extracts_selected_artifact_prediction(self):
        request = self._request()
        request["model_artifact"] = {
            "predictions": [
                {
                    "row_id": request["row_id"],
                    "target": runtime.TARGET,
                    "horizon_days": runtime.HORIZON_DAYS,
                    "source_cutoff_date": request["observation_date"],
                    "status": runtime.STATUS_PREDICTED,
                    "stockout_risk": 0.4,
                    "uncertainty": {
                        "method": "wilson-95",
                        "lower": 0.1,
                        "upper": 0.7,
                        "sample_count": 10,
                    },
                    "abstention_reason": "",
                }
            ]
        }
        response = runtime.build_response(request)
        self.assertEqual(response["status"], runtime.STATUS_PREDICTED)
        self.assertEqual(response["stockout_risk"], 0.4)

    def test_missing_artifact_abstains(self):
        response = runtime.build_response(self._request())
        self.assertEqual(response["status"], runtime.STATUS_ABSTAINED)
        self.assertEqual(response["abstention_reason"], runtime.ABSTENTION_UNSUPPORTED_INPUT)

    @staticmethod
    def _request():
        return {
            "contract_version": runtime.CONTRACT_VERSION,
            "predictor": runtime.PREDICTOR,
            "tenant_id": "11111111-1111-4111-8111-111111111111",
            "item_id": "item-flour-001",
            "row_id": "row-1",
            "observation_date": "2026-01-12",
            "target": runtime.TARGET,
            "horizon_days": runtime.HORIZON_DAYS,
            "feature_definition_version": "m4-raw-onhand-v1",
            "feature_snapshot_id": "snapshot-1",
            "feature_snapshot_checksum": "checksum-1",
            "model_version": "m4-onhand-bucket-rate-v1",
            "code_version": "m4-python-stockout-candidate-v1",
            "feature": {
                "row_id": "row-1",
                "tenant_id": "11111111-1111-4111-8111-111111111111",
                "item_id": "item-flour-001",
                "as_of_date": "2026-01-12",
                "source_cutoff_date": "2026-01-12",
            },
        }


if __name__ == "__main__":
    unittest.main()
