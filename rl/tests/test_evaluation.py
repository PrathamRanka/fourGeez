"""Tests for paired offline baseline comparison and regression reporting."""

from __future__ import annotations

import json
import unittest

from agentpay_rl.evaluation import (
    EVALUATION_SCHEMA_VERSION,
    EvaluationThresholds,
    OffPolicyEvaluationError,
    PairedEvaluationSample,
    StrategyOutcome,
    evaluate_paired_samples,
)


# sample creates one paired observation with explicit outcome support.
def sample(
    sample_id: str,
    segment: str,
    baseline_reward: float,
    candidate_reward: float,
    candidate_observed: bool = True,
) -> PairedEvaluationSample:
    return PairedEvaluationSample(
        sample_id=sample_id,
        segment=segment,
        synthetic=True,
        baseline=StrategyOutcome(
            accepted=True,
            amount_atomic=500,
            fulfilled=True,
            disputed=False,
            reward=baseline_reward,
        ),
        candidate=StrategyOutcome(
            accepted=True,
            amount_atomic=450,
            fulfilled=True,
            disputed=False,
            reward=candidate_reward,
        ),
        candidate_outcome_observed=candidate_observed,
    )


class EvaluationTests(unittest.TestCase):
    # test_report_contains_metrics_deltas_intervals_and_segments verifies completeness.
    def test_report_contains_metrics_deltas_intervals_and_segments(self) -> None:
        samples = tuple(
            sample(
                sample_id=f"sample_{index}",
                segment=("cost-sensitive", "quality-sensitive")[index % 2],
                baseline_reward=0.8,
                candidate_reward=1.0,
            )
            for index in range(40)
        )

        report = evaluate_paired_samples(
            samples,
            EvaluationThresholds.default(),
        )

        self.assertEqual(report.schema_version, EVALUATION_SCHEMA_VERSION)
        self.assertTrue(report.synthetic)
        self.assertTrue(report.passes_thresholds)
        self.assertAlmostEqual(report.metrics["aggregate_reward"].delta, 0.2)
        self.assertAlmostEqual(report.metrics["average_cost_atomic"].delta, -50.0)
        self.assertLessEqual(
            report.metrics["aggregate_reward"].confidence_low,
            report.metrics["aggregate_reward"].delta,
        )
        self.assertEqual(
            set(report.segment_metrics),
            {"cost-sensitive", "quality-sensitive"},
        )
        self.assertEqual(json.loads(report.to_json())["synthetic"], True)
        self.assertIn("SYNTHETIC DATA", report.to_markdown())

    # test_regression_thresholds_fail_the_report verifies release gating.
    def test_regression_thresholds_fail_the_report(self) -> None:
        samples = tuple(
            sample(
                sample_id=f"sample_{index}",
                segment="cost-sensitive",
                baseline_reward=1.0,
                candidate_reward=0.5,
            )
            for index in range(40)
        )

        report = evaluate_paired_samples(
            samples,
            EvaluationThresholds(
                minimum_reward_delta=-0.05,
                maximum_dispute_rate_delta=0.01,
            ),
        )

        self.assertFalse(report.passes_thresholds)
        self.assertIn("aggregate_reward", report.regressions)

    # test_small_samples_emit_warning verifies confidence limitations are visible.
    def test_small_samples_emit_warning(self) -> None:
        report = evaluate_paired_samples(
            (sample("sample_small", "latency-sensitive", 0.8, 0.9),),
            EvaluationThresholds.default(),
        )

        self.assertTrue(any("sample" in warning for warning in report.warnings))

    # test_unobserved_candidate_outcomes_are_rejected blocks off-policy claims.
    def test_unobserved_candidate_outcomes_are_rejected(self) -> None:
        unsupported = sample(
            "sample_unobserved",
            "quality-sensitive",
            0.8,
            1.0,
            candidate_observed=False,
        )

        with self.assertRaises(OffPolicyEvaluationError):
            evaluate_paired_samples(
                (unsupported,),
                EvaluationThresholds.default(),
            )

    # test_mixed_provenance_is_rejected prevents synthetic results appearing as real.
    def test_mixed_provenance_is_rejected(self) -> None:
        real_sample = PairedEvaluationSample(
            sample_id="sample_real",
            segment="quality-sensitive",
            synthetic=False,
            baseline=sample("base", "quality-sensitive", 0.8, 1.0).baseline,
            candidate=sample("candidate", "quality-sensitive", 0.8, 1.0).candidate,
            candidate_outcome_observed=True,
        )

        with self.assertRaisesRegex(ValueError, "provenance"):
            evaluate_paired_samples(
                (sample("sample_synthetic", "cost-sensitive", 0.8, 1.0), real_sample),
                EvaluationThresholds.default(),
            )


if __name__ == "__main__":
    unittest.main()
