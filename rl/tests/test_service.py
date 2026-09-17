"""Tests for the bounded JSON recommendation adapter."""

from __future__ import annotations

import json
import unittest

from rl.tests.test_contracts import context_payload

from agentpay_rl.baseline import DeterministicBaseline
from agentpay_rl.contracts import (
    FEATURE_VERSION_V1,
    RECOMMENDATION_SCHEMA_VERSION,
    DecisionContext,
    Recommendation,
)
from agentpay_rl.features import FeatureBatch
from agentpay_rl.service import RecommendationService, ServiceError


class UnsafeRanker:
    """Returns an undocumented offer to verify output revalidation."""

    strategy = "unsafe-ranker"
    model_version = "unsafe-v1"

    # rank deliberately violates the ranking contract for one test.
    def rank(
        self,
        context: DecisionContext,
        batch: FeatureBatch,
    ) -> Recommendation:
        return Recommendation(
            schema_version=RECOMMENDATION_SCHEMA_VERSION,
            recommendation_id="rec_unsafe",
            request_id=context.request_id,
            ranked_offer_ids=("offer_not_supplied",),
            scores={"offer_not_supplied": 1.0},
            reasons=("unsafe test output",),
            strategy=self.strategy,
            model_version=self.model_version,
            feature_version=FEATURE_VERSION_V1,
        )


class CountingRanker:
    """Records whether an oversized request reached model execution."""

    strategy = "counting-ranker"
    model_version = "counting-v1"

    # __init__ starts with no ranking calls.
    def __init__(self) -> None:
        self.call_count = 0

    # rank increments the call count and delegates to the baseline.
    def rank(
        self,
        context: DecisionContext,
        batch: FeatureBatch,
    ) -> Recommendation:
        self.call_count += 1
        return DeterministicBaseline().rank(context, batch)


class SequenceClock:
    """Returns deterministic monotonic values for timeout tests."""

    # __init__ stores the ordered values returned by calls.
    def __init__(self, values: tuple[float, ...]) -> None:
        self._values = iter(values)

    # __call__ returns the next configured monotonic timestamp.
    def __call__(self) -> float:
        return next(self._values)


class ServiceTests(unittest.TestCase):
    # test_valid_json_returns_revalidated_recommendation verifies the adapter path.
    def test_valid_json_returns_revalidated_recommendation(self) -> None:
        service = RecommendationService(DeterministicBaseline())
        request_body = json.dumps(context_payload()).encode("utf-8")

        response = json.loads(service.recommend(request_body))

        self.assertEqual(response["requestId"], "request_alpha")
        self.assertEqual(response["rankedOfferIds"], ["offer_alpha"])

    # test_oversized_request_is_rejected_before_ranking verifies byte limits.
    def test_oversized_request_is_rejected_before_ranking(self) -> None:
        ranker = CountingRanker()
        service = RecommendationService(ranker, maximum_request_bytes=8)

        with self.assertRaisesRegex(ServiceError, "request_too_large"):
            service.recommend(json.dumps(context_payload()).encode("utf-8"))

        self.assertEqual(ranker.call_count, 0)

    # test_timeout_is_reported verifies bounded execution time.
    def test_timeout_is_reported(self) -> None:
        service = RecommendationService(
            DeterministicBaseline(),
            timeout_seconds=0.1,
            monotonic_clock=SequenceClock((10.0, 10.2)),
        )

        with self.assertRaisesRegex(ServiceError, "ranking_timeout"):
            service.recommend(json.dumps(context_payload()).encode("utf-8"))

    # test_unknown_ranked_offer_is_rejected protects the Go integration boundary.
    def test_unknown_ranked_offer_is_rejected(self) -> None:
        service = RecommendationService(UnsafeRanker())

        with self.assertRaisesRegex(ServiceError, "invalid_recommendation"):
            service.recommend(json.dumps(context_payload()).encode("utf-8"))

    # test_health_and_metadata_do_not_expose_model_state verifies safe operations.
    def test_health_and_metadata_do_not_expose_model_state(self) -> None:
        service = RecommendationService(DeterministicBaseline())

        self.assertEqual(service.health(), {"status": "ok"})
        metadata = service.model_metadata()
        self.assertEqual(metadata["strategy"], "deterministic-baseline")
        self.assertNotIn("weights", metadata)
        self.assertNotIn("trainingData", metadata)


if __name__ == "__main__":
    unittest.main()
