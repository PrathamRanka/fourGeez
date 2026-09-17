"""Tests for reproducible, explicitly synthetic recommendation datasets."""

from __future__ import annotations

import unittest

from agentpay_rl.simulator import (
    DATASET_SCHEMA_VERSION,
    MAXIMUM_SYNTHETIC_SCENARIOS,
    generate_synthetic_dataset,
)


class SimulatorTests(unittest.TestCase):
    # test_same_seed_produces_identical_dataset verifies reproducibility.
    def test_same_seed_produces_identical_dataset(self) -> None:
        first = generate_synthetic_dataset(seed=17, scenario_count=9)
        second = generate_synthetic_dataset(seed=17, scenario_count=9)

        self.assertEqual(first, second)
        self.assertEqual(first.to_json(), second.to_json())

    # test_different_seed_changes_dataset verifies the seed controls generation.
    def test_different_seed_changes_dataset(self) -> None:
        first = generate_synthetic_dataset(seed=17, scenario_count=9)
        second = generate_synthetic_dataset(seed=18, scenario_count=9)

        self.assertNotEqual(first.fingerprint, second.fingerprint)

    # test_dataset_is_labeled_and_covers_segments verifies honest reporting metadata.
    def test_dataset_is_labeled_and_covers_segments(self) -> None:
        dataset = generate_synthetic_dataset(seed=23, scenario_count=9)

        self.assertTrue(dataset.synthetic)
        self.assertEqual(dataset.schema_version, DATASET_SCHEMA_VERSION)
        self.assertEqual(dataset.seed, 23)
        self.assertEqual(
            {scenario.segment for scenario in dataset.scenarios},
            {"cost-sensitive", "quality-sensitive", "latency-sensitive"},
        )

    # test_outcomes_reference_generated_recommendations verifies event integrity.
    def test_outcomes_reference_generated_recommendations(self) -> None:
        dataset = generate_synthetic_dataset(seed=31, scenario_count=12)

        for scenario in dataset.scenarios:
            recommendation = scenario.recommendation
            for outcome_event in scenario.outcome_events:
                self.assertEqual(
                    outcome_event.recommendation_id,
                    recommendation.recommendation_id,
                )
                self.assertIn(outcome_event.offer_id, recommendation.ranked_offer_ids)
                self.assertEqual(
                    outcome_event.feature_version,
                    scenario.context.feature_version,
                )

    # test_invalid_scenario_counts_are_rejected verifies bounded generation.
    def test_invalid_scenario_counts_are_rejected(self) -> None:
        for scenario_count in (0, -1, MAXIMUM_SYNTHETIC_SCENARIOS + 1):
            with (
                self.subTest(scenario_count=scenario_count),
                self.assertRaisesRegex(ValueError, "scenario_count"),
            ):
                generate_synthetic_dataset(seed=1, scenario_count=scenario_count)


if __name__ == "__main__":
    unittest.main()
