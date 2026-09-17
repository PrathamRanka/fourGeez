"""Tests for versioned and auditable reward construction."""

from __future__ import annotations

import unittest

from agentpay_rl.contracts import (
    FEATURE_VERSION_V1,
    OUTCOME_EVENT_SCHEMA_VERSION,
    OutcomeEvent,
    OutcomeType,
)
from agentpay_rl.rewards import (
    REWARD_CONFIG_VERSION,
    IncompleteOutcomeSequence,
    RewardConfig,
    calculate_reward,
)


# outcome creates one version-compatible event at a deterministic timestamp.
def outcome(event_type: OutcomeType, event_index: int) -> OutcomeEvent:
    return OutcomeEvent(
        schema_version=OUTCOME_EVENT_SCHEMA_VERSION,
        event_id=f"event_reward_{event_index}",
        recommendation_id="rec_reward",
        offer_id="offer_reward",
        event_type=event_type,
        occurred_at=f"2026-01-01T00:00:0{event_index}Z",
        feature_version=FEATURE_VERSION_V1,
        strategy_version="deterministic-baseline",
        model_version="baseline-v1",
    )


class RewardTests(unittest.TestCase):
    # test_reward_returns_each_component_and_total verifies auditability.
    def test_reward_returns_each_component_and_total(self) -> None:
        events = (
            outcome(OutcomeType.RECOMMENDATION_ACCEPTED, 0),
            outcome(OutcomeType.TRANSACTION_FULFILLED, 1),
            outcome(OutcomeType.TRANSACTION_DISPUTED, 2),
            outcome(OutcomeType.DISPUTE_RESOLVED, 3),
        )

        result = calculate_reward(events, RewardConfig.default())

        self.assertEqual(result.config_version, REWARD_CONFIG_VERSION)
        self.assertEqual(
            tuple(component.event_type for component in result.components),
            tuple(event.event_type for event in events),
        )
        self.assertAlmostEqual(
            result.total,
            sum(component.value for component in result.components),
        )

    # test_custom_weights_are_explicit verifies configuration controls scoring.
    def test_custom_weights_are_explicit(self) -> None:
        config = RewardConfig(
            version=REWARD_CONFIG_VERSION,
            accepted=0.25,
            overridden=-0.5,
            fulfilled=2.0,
            failed=-2.0,
            disputed=-1.0,
            resolved=0.0,
        )

        result = calculate_reward(
            (
                outcome(OutcomeType.RECOMMENDATION_OVERRIDDEN, 0),
                outcome(OutcomeType.TRANSACTION_FAILED, 1),
            ),
            config,
        )

        self.assertEqual(result.total, -2.5)

    # test_incomplete_or_invalid_sequences_fail_closed prevents future leakage.
    def test_incomplete_or_invalid_sequences_fail_closed(self) -> None:
        invalid_sequences = (
            (outcome(OutcomeType.RECOMMENDATION_ACCEPTED, 0),),
            (
                outcome(OutcomeType.TRANSACTION_FULFILLED, 0),
                outcome(OutcomeType.RECOMMENDATION_ACCEPTED, 1),
            ),
            (
                outcome(OutcomeType.RECOMMENDATION_ACCEPTED, 0),
                outcome(OutcomeType.TRANSACTION_DISPUTED, 1),
            ),
        )

        for events in invalid_sequences:
            with (
                self.subTest(events=events),
                self.assertRaises(IncompleteOutcomeSequence),
            ):
                calculate_reward(events, RewardConfig.default())

    # test_events_after_cutoff_are_rejected prevents training on future outcomes.
    def test_events_after_cutoff_are_rejected(self) -> None:
        events = (
            outcome(OutcomeType.RECOMMENDATION_ACCEPTED, 0),
            outcome(OutcomeType.TRANSACTION_FULFILLED, 1),
        )

        with self.assertRaisesRegex(IncompleteOutcomeSequence, "cutoff"):
            calculate_reward(
                events,
                RewardConfig.default(),
                as_of="2026-01-01T00:00:00Z",
            )

    # test_mixed_offer_events_are_rejected prevents cross-transaction rewards.
    def test_mixed_offer_events_are_rejected(self) -> None:
        first = outcome(OutcomeType.RECOMMENDATION_ACCEPTED, 0)
        second = OutcomeEvent(
            schema_version=first.schema_version,
            event_id="event_other",
            recommendation_id=first.recommendation_id,
            offer_id="offer_other",
            event_type=OutcomeType.TRANSACTION_FULFILLED,
            occurred_at="2026-01-01T00:00:01Z",
            feature_version=first.feature_version,
            strategy_version=first.strategy_version,
            model_version=first.model_version,
        )

        with self.assertRaisesRegex(IncompleteOutcomeSequence, "offer"):
            calculate_reward((first, second), RewardConfig.default())


if __name__ == "__main__":
    unittest.main()
