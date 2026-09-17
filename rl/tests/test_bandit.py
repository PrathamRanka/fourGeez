"""Tests for offline contextual-bandit training and read-only ranking."""

from __future__ import annotations

import json
import math
import unittest

from agentpay_rl.bandit import (
    BANDIT_MODEL_VERSION,
    IncompatibleModelVersion,
    InvalidTrainingExample,
    LinUCBBandit,
    OfflineTrainingExample,
)
from agentpay_rl.contracts import (
    CANDIDATE_OFFER_SCHEMA_VERSION,
    DECISION_CONTEXT_SCHEMA_VERSION,
    FEATURE_VERSION_V1,
    CandidateOffer,
    DecisionContext,
)
from agentpay_rl.features import FEATURE_DIM, build_feature_batch


# context creates one stable ranking request for model tests.
def context() -> DecisionContext:
    candidates = (
        CandidateOffer(
            schema_version=CANDIDATE_OFFER_SCHEMA_VERSION,
            offer_id="offer_alpha",
            seller_id="seller_alpha",
            amount="200",
            asset="USDC",
            network="eip155:84532",
            capabilities=("research",),
            available=True,
            eligible=True,
            historical_delivery_rate_bps=9900,
            historical_dispute_rate_bps=50,
            p95_latency_ms=800,
        ),
        CandidateOffer(
            schema_version=CANDIDATE_OFFER_SCHEMA_VERSION,
            offer_id="offer_beta",
            seller_id="seller_beta",
            amount="700",
            asset="USDC",
            network="eip155:84532",
            capabilities=("research",),
            available=True,
            eligible=True,
            historical_delivery_rate_bps=8500,
            historical_dispute_rate_bps=700,
            p95_latency_ms=5000,
        ),
    )
    return DecisionContext(
        schema_version=DECISION_CONTEXT_SCHEMA_VERSION,
        request_id="request_bandit",
        feature_version=FEATURE_VERSION_V1,
        maximum_amount="1000",
        price_weight_bps=5000,
        quality_weight_bps=3000,
        latency_weight_bps=2000,
        candidates=candidates,
    )


# training_examples creates deterministic positive and negative observations.
def training_examples() -> tuple[OfflineTrainingExample, ...]:
    return (
        OfflineTrainingExample(
            event_id="training_positive",
            feature_version=FEATURE_VERSION_V1,
            features=(0.5, 0.3, 0.2, 0.8, 0.99, 0.995, 0.997),
            reward=1.1,
        ),
        OfflineTrainingExample(
            event_id="training_negative",
            feature_version=FEATURE_VERSION_V1,
            features=(0.5, 0.3, 0.2, 0.3, 0.85, 0.93, 0.983),
            reward=-1.1,
        ),
    )


class BanditTests(unittest.TestCase):
    # test_training_is_deterministic_and_ranker_compatible verifies the common boundary.
    def test_training_is_deterministic_and_ranker_compatible(self) -> None:
        first = LinUCBBandit(seed=41)
        second = LinUCBBandit(seed=41)

        first.fit_offline(training_examples())
        second.fit_offline(training_examples())
        decision_context = context()
        feature_batch = build_feature_batch(decision_context)

        self.assertEqual(
            first.rank(decision_context, feature_batch),
            second.rank(decision_context, feature_batch),
        )
        self.assertEqual(first.model_version, BANDIT_MODEL_VERSION)

    # test_rank_does_not_update_model_state protects the live request path.
    def test_rank_does_not_update_model_state(self) -> None:
        model = LinUCBBandit(seed=7)
        model.fit_offline(training_examples())
        before = model.metadata()
        decision_context = context()

        recommendation = model.rank(
            decision_context,
            build_feature_batch(decision_context),
        )

        self.assertEqual(model.metadata(), before)
        self.assertTrue(all(math.isfinite(score) for score in recommendation.scores.values()))

    # test_json_artifact_round_trip_preserves_ranking verifies portable persistence.
    def test_json_artifact_round_trip_preserves_ranking(self) -> None:
        model = LinUCBBandit(seed=13)
        model.fit_offline(training_examples())
        artifact = model.to_json()
        loaded = LinUCBBandit.from_json(artifact)
        decision_context = context()
        feature_batch = build_feature_batch(decision_context)

        self.assertEqual(
            loaded.rank(decision_context, feature_batch),
            model.rank(decision_context, feature_batch),
        )
        self.assertEqual(json.loads(loaded.to_json()), json.loads(artifact))

    # test_incompatible_artifact_versions_fail_closed verifies version pinning.
    def test_incompatible_artifact_versions_fail_closed(self) -> None:
        model = LinUCBBandit(seed=13)
        artifact = json.loads(model.to_json())
        artifact["metadata"]["featureVersion"] = "agentpay.features.v2"

        with self.assertRaises(IncompatibleModelVersion):
            LinUCBBandit.from_json(json.dumps(artifact))

    # test_invalid_batch_never_mutates_weights verifies validation before fitting.
    def test_invalid_batch_never_mutates_weights(self) -> None:
        model = LinUCBBandit(seed=19)
        before = model.metadata()
        invalid_example = OfflineTrainingExample(
            event_id="training_invalid",
            feature_version=FEATURE_VERSION_V1,
            features=tuple(0.0 for _ in range(FEATURE_DIM - 1)),
            reward=1.0,
        )

        with self.assertRaises(InvalidTrainingExample):
            model.fit_offline((invalid_example,))

        self.assertEqual(model.metadata(), before)


if __name__ == "__main__":
    unittest.main()
