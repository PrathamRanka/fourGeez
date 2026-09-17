"""Tests for the published R1 evaluation report and provenance metadata."""

from __future__ import annotations

import json
import unittest
from pathlib import Path

from agentpay_rl.bandit import BANDIT_MODEL_VERSION, BANDIT_STRATEGY_NAME
from agentpay_rl.contracts import FEATURE_VERSION_V1
from agentpay_rl.evaluation import EVALUATION_SCHEMA_VERSION
from agentpay_rl.reporting import EVALUATION_PUBLICATION_VERSION
from agentpay_rl.rewards import REWARD_CONFIG_VERSION
from agentpay_rl.simulator import DATASET_SCHEMA_VERSION

REPORT_DIRECTORY = Path(__file__).resolve().parents[1] / "reports"
REPORT_JSON_PATH = REPORT_DIRECTORY / "r1-evaluation.json"
REPORT_MARKDOWN_PATH = REPORT_DIRECTORY / "r1-evaluation.md"


class ReportingTests(unittest.TestCase):
    # test_published_report_is_synthetic_versioned_and_passing verifies R1 evidence.
    def test_published_report_is_synthetic_versioned_and_passing(self) -> None:
        publication = json.loads(REPORT_JSON_PATH.read_text(encoding="utf-8"))

        self.assertEqual(publication["schemaVersion"], EVALUATION_PUBLICATION_VERSION)
        self.assertTrue(publication["synthetic"])
        self.assertEqual(publication["datasetVersion"], DATASET_SCHEMA_VERSION)
        self.assertEqual(publication["featureVersion"], FEATURE_VERSION_V1)
        self.assertEqual(publication["rewardConfigVersion"], REWARD_CONFIG_VERSION)
        self.assertEqual(publication["candidateStrategy"], BANDIT_STRATEGY_NAME)
        self.assertEqual(publication["candidateModelVersion"], BANDIT_MODEL_VERSION)
        self.assertEqual(
            publication["evaluation"]["schemaVersion"],
            EVALUATION_SCHEMA_VERSION,
        )
        self.assertTrue(publication["evaluation"]["passesThresholds"])
        self.assertGreaterEqual(
            publication["evaluation"]["metrics"]["aggregate_reward"]["delta"],
            0.0,
        )

    # test_markdown_report_cannot_be_mistaken_for_customer_results verifies labeling.
    def test_markdown_report_cannot_be_mistaken_for_customer_results(self) -> None:
        report = REPORT_MARKDOWN_PATH.read_text(encoding="utf-8")

        self.assertIn("SYNTHETIC DATA", report)
        self.assertIn("NOT CUSTOMER PERFORMANCE", report)
        self.assertIn("Dataset fingerprint", report)
        self.assertIn("Model fingerprint", report)


if __name__ == "__main__":
    unittest.main()
