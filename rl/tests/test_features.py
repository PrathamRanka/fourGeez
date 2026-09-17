"""Tests for strict candidate validation and deterministic feature construction."""

from __future__ import annotations

import unittest

from agentpay_rl.contracts import (
    CANDIDATE_OFFER_SCHEMA_VERSION,
    DECISION_CONTEXT_SCHEMA_VERSION,
    FEATURE_VERSION_V1,
    CandidateOffer,
    DecisionContext,
)
from agentpay_rl.features import FEATURE_NAMES, RejectionReason, build_feature_batch


# candidate creates one valid candidate with explicit ranking facts.
def candidate(
    offer_id: str,
    amount: str = "125",
    available: bool = True,
    eligible: bool = True,
) -> CandidateOffer:
    return CandidateOffer(
        schema_version=CANDIDATE_OFFER_SCHEMA_VERSION,
        offer_id=offer_id,
        seller_id=f"seller_{offer_id}",
        amount=amount,
        asset="USDC",
        network="eip155:84532",
        capabilities=("research", "json"),
        available=available,
        eligible=eligible,
        historical_delivery_rate_bps=9800,
        historical_dispute_rate_bps=75,
        p95_latency_ms=850,
    )


# decision_context creates a strict input with caller-selected candidates.
def decision_context(candidates: tuple[CandidateOffer, ...]) -> DecisionContext:
    return DecisionContext(
        schema_version=DECISION_CONTEXT_SCHEMA_VERSION,
        request_id="request_features",
        feature_version=FEATURE_VERSION_V1,
        buyer_segment_id="segment_cost_sensitive",
        maximum_amount="500",
        price_weight_bps=5000,
        quality_weight_bps=3000,
        latency_weight_bps=2000,
        candidates=candidates,
    )


class FeatureTests(unittest.TestCase):
    # test_build_feature_batch_filters_candidates_before_ranking verifies eligibility.
    def test_build_feature_batch_filters_candidates_before_ranking(self) -> None:
        context = decision_context(
            (
                candidate("offer_valid"),
                candidate("offer_unavailable", available=False),
                candidate("offer_ineligible", eligible=False),
                candidate("offer_expensive", amount="501"),
            )
        )

        batch = build_feature_batch(context)

        self.assertEqual(batch.candidate_ids, ("offer_valid",))
        self.assertEqual(
            tuple((rejection.offer_id, rejection.reason) for rejection in batch.rejected),
            (
                ("offer_expensive", RejectionReason.OVER_BUDGET),
                ("offer_ineligible", RejectionReason.INELIGIBLE),
                ("offer_unavailable", RejectionReason.UNAVAILABLE),
            ),
        )

    # test_build_feature_batch_matches_golden_vector verifies exact normalization.
    def test_build_feature_batch_matches_golden_vector(self) -> None:
        batch = build_feature_batch(decision_context((candidate("offer_alpha"),)))

        self.assertEqual(
            FEATURE_NAMES,
            (
                "price_weight",
                "quality_weight",
                "latency_weight",
                "price_score",
                "delivery_rate",
                "dispute_free_rate",
                "latency_score",
            ),
        )
        self.assertEqual(batch.feature_names, FEATURE_NAMES)
        self.assertEqual(batch.feature_version, FEATURE_VERSION_V1)
        self.assertEqual(batch.candidate_ids, ("offer_alpha",))
        expected = (0.5, 0.3, 0.2, 0.75, 0.98, 0.9925, 0.9971666666666666)
        for actual_value, expected_value in zip(batch.rows[0], expected, strict=True):
            self.assertAlmostEqual(actual_value, expected_value)

    # test_candidate_order_does_not_change_feature_output verifies determinism.
    def test_candidate_order_does_not_change_feature_output(self) -> None:
        first = build_feature_batch(
            decision_context((candidate("offer_beta"), candidate("offer_alpha")))
        )
        second = build_feature_batch(
            decision_context((candidate("offer_alpha"), candidate("offer_beta")))
        )

        self.assertEqual(first, second)
        self.assertEqual(first.candidate_ids, ("offer_alpha", "offer_beta"))

    # test_exact_budget_candidate_is_eligible verifies the inclusive ceiling.
    def test_exact_budget_candidate_is_eligible(self) -> None:
        batch = build_feature_batch(decision_context((candidate("offer_boundary", amount="500"),)))

        self.assertEqual(batch.candidate_ids, ("offer_boundary",))
        self.assertEqual(batch.rows[0][3], 0.0)


if __name__ == "__main__":
    unittest.main()
