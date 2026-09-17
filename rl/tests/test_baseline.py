"""Golden tests for deterministic ranking behavior."""

from __future__ import annotations

import unittest

from agentpay_rl.baseline import DeterministicBaseline, NoRankableCandidates
from agentpay_rl.contracts import (
    CANDIDATE_OFFER_SCHEMA_VERSION,
    DECISION_CONTEXT_SCHEMA_VERSION,
    FEATURE_VERSION_V1,
    CandidateOffer,
    DecisionContext,
)
from agentpay_rl.features import build_feature_batch


# offer creates one candidate with controllable ranking dimensions.
def offer(
    offer_id: str,
    amount: str,
    delivery_rate_bps: int,
    dispute_rate_bps: int,
    latency_ms: int,
    eligible: bool = True,
) -> CandidateOffer:
    return CandidateOffer(
        schema_version=CANDIDATE_OFFER_SCHEMA_VERSION,
        offer_id=offer_id,
        seller_id=f"seller_{offer_id}",
        amount=amount,
        asset="USDC",
        network="eip155:84532",
        capabilities=("research",),
        available=True,
        eligible=eligible,
        historical_delivery_rate_bps=delivery_rate_bps,
        historical_dispute_rate_bps=dispute_rate_bps,
        p95_latency_ms=latency_ms,
    )


# context creates one versioned baseline request.
def context(candidates: tuple[CandidateOffer, ...]) -> DecisionContext:
    return DecisionContext(
        schema_version=DECISION_CONTEXT_SCHEMA_VERSION,
        request_id="request_baseline",
        feature_version=FEATURE_VERSION_V1,
        maximum_amount="1000",
        price_weight_bps=5000,
        quality_weight_bps=3000,
        latency_weight_bps=2000,
        candidates=candidates,
    )


class BaselineTests(unittest.TestCase):
    # test_rank_matches_golden_order_and_scores verifies the published formula.
    def test_rank_matches_golden_order_and_scores(self) -> None:
        decision_context = context(
            (
                offer("offer_quality", "700", 10_000, 0, 1200),
                offer("offer_value", "200", 9500, 100, 900),
            )
        )

        recommendation = DeterministicBaseline().rank(
            decision_context,
            build_feature_batch(decision_context),
        )

        self.assertEqual(
            recommendation.ranked_offer_ids,
            ("offer_value", "offer_quality"),
        )
        self.assertAlmostEqual(recommendation.scores["offer_value"], 0.8904)
        self.assertAlmostEqual(recommendation.scores["offer_quality"], 0.6492)
        self.assertEqual(recommendation.strategy, "deterministic-baseline")
        self.assertEqual(recommendation.model_version, "baseline-v1")
        self.assertIn("price fit", recommendation.reasons[0])

    # test_ties_and_recommendation_id_ignore_candidate_input_order verifies determinism.
    def test_ties_and_recommendation_id_ignore_candidate_input_order(self) -> None:
        offer_alpha = offer("offer_alpha", "500", 9000, 100, 1000)
        offer_beta = offer("offer_beta", "500", 9000, 100, 1000)
        first_context = context((offer_beta, offer_alpha))
        second_context = context((offer_alpha, offer_beta))

        first = DeterministicBaseline().rank(
            first_context,
            build_feature_batch(first_context),
        )
        second = DeterministicBaseline().rank(
            second_context,
            build_feature_batch(second_context),
        )

        self.assertEqual(first, second)
        self.assertEqual(first.ranked_offer_ids, ("offer_alpha", "offer_beta"))
        self.assertTrue(first.recommendation_id.startswith("rec_"))

    # test_rank_excludes_candidates_rejected_by_feature_validation verifies the boundary.
    def test_rank_excludes_candidates_rejected_by_feature_validation(self) -> None:
        decision_context = context(
            (
                offer("offer_valid", "500", 9000, 100, 1000),
                offer("offer_ineligible", "100", 10_000, 0, 100, eligible=False),
            )
        )

        recommendation = DeterministicBaseline().rank(
            decision_context,
            build_feature_batch(decision_context),
        )

        self.assertEqual(recommendation.ranked_offer_ids, ("offer_valid",))
        self.assertNotIn("offer_ineligible", recommendation.scores)

    # test_rank_rejects_empty_feature_batch avoids fabricating a recommendation.
    def test_rank_rejects_empty_feature_batch(self) -> None:
        decision_context = context(
            (offer("offer_ineligible", "100", 10_000, 0, 100, eligible=False),)
        )

        with self.assertRaises(NoRankableCandidates):
            DeterministicBaseline().rank(
                decision_context,
                build_feature_batch(decision_context),
            )


if __name__ == "__main__":
    unittest.main()
