"""Contract tests for strict validation and serialization compatibility."""

from __future__ import annotations

import json
import math
import unittest

from agentpay_rl.contracts import (
    CANDIDATE_OFFER_SCHEMA_VERSION,
    DECISION_CONTEXT_SCHEMA_VERSION,
    FEATURE_VERSION_V1,
    OUTCOME_EVENT_SCHEMA_VERSION,
    RECOMMENDATION_SCHEMA_VERSION,
    CandidateOffer,
    ContractError,
    DecisionContext,
    OutcomeEvent,
    Recommendation,
)


# candidate_payload returns one complete eligible-offer fixture.
def candidate_payload() -> dict[str, object]:
    return {
        "schemaVersion": CANDIDATE_OFFER_SCHEMA_VERSION,
        "offerId": "offer_alpha",
        "sellerId": "seller_alpha",
        "amount": "125",
        "asset": "USDC",
        "network": "eip155:84532",
        "capabilities": ["research", "json"],
        "available": True,
        "eligible": True,
        "historicalDeliveryRateBps": 9800,
        "historicalDisputeRateBps": 75,
        "p95LatencyMs": 850,
    }


# context_payload returns one complete recommendation-input fixture.
def context_payload() -> dict[str, object]:
    return {
        "schemaVersion": DECISION_CONTEXT_SCHEMA_VERSION,
        "requestId": "request_alpha",
        "featureVersion": FEATURE_VERSION_V1,
        "buyerSegmentId": "segment_cost_sensitive",
        "maximumAmount": "500",
        "priceWeightBps": 5000,
        "qualityWeightBps": 3000,
        "latencyWeightBps": 2000,
        "candidates": [candidate_payload()],
    }


class ContractTests(unittest.TestCase):
    # test_decision_context_round_trip_preserves_wire_shape verifies stable JSON.
    def test_decision_context_round_trip_preserves_wire_shape(self) -> None:
        payload = context_payload()
        context = DecisionContext.from_dict(payload)
        candidate = CandidateOffer.from_dict(candidate_payload())

        self.assertEqual(context.to_dict(), payload)
        self.assertEqual(
            DecisionContext.from_json(context.to_json()),
            context,
        )
        self.assertEqual(
            CandidateOffer.from_json(candidate.to_json()),
            candidate,
        )

    # test_recommendation_round_trip_validates_ranked_scores verifies output compatibility.
    def test_recommendation_round_trip_validates_ranked_scores(self) -> None:
        payload = {
            "schemaVersion": RECOMMENDATION_SCHEMA_VERSION,
            "recommendationId": "rec_alpha",
            "requestId": "request_alpha",
            "rankedOfferIds": ["offer_alpha", "offer_beta"],
            "scores": {"offer_alpha": 0.82, "offer_beta": 0.61},
            "reasons": ["within budget", "higher delivery reliability"],
            "strategy": "deterministic-baseline",
            "modelVersion": "baseline-v1",
            "featureVersion": FEATURE_VERSION_V1,
        }

        recommendation = Recommendation.from_dict(payload)

        self.assertEqual(recommendation.to_dict(), payload)
        self.assertEqual(
            Recommendation.from_json(recommendation.to_json()),
            recommendation,
        )

    # test_outcome_event_round_trip_requires_utc verifies training-event compatibility.
    def test_outcome_event_round_trip_requires_utc(self) -> None:
        payload = {
            "schemaVersion": OUTCOME_EVENT_SCHEMA_VERSION,
            "eventId": "event_alpha",
            "recommendationId": "rec_alpha",
            "offerId": "offer_alpha",
            "eventType": "transaction.fulfilled",
            "occurredAt": "2026-09-17T10:00:00Z",
            "featureVersion": FEATURE_VERSION_V1,
            "strategyVersion": "deterministic-baseline-v1",
            "modelVersion": "baseline-v1",
        }

        event = OutcomeEvent.from_dict(payload)

        self.assertEqual(event.to_dict(), payload)
        self.assertEqual(OutcomeEvent.from_json(event.to_json()), event)

        non_utc_payload = dict(payload)
        non_utc_payload["occurredAt"] = "2026-09-17T15:30:00+05:30"
        with self.assertRaisesRegex(ContractError, "occurredAt"):
            OutcomeEvent.from_dict(non_utc_payload)

    # test_unknown_and_missing_fields_fail_closed verifies strict object schemas.
    def test_unknown_and_missing_fields_fail_closed(self) -> None:
        unknown_payload = context_payload()
        unknown_payload["trustTier"] = "gold"
        with self.assertRaisesRegex(ContractError, "unknown field"):
            DecisionContext.from_dict(unknown_payload)

        missing_payload = context_payload()
        del missing_payload["schemaVersion"]
        with self.assertRaisesRegex(ContractError, "missing field"):
            DecisionContext.from_dict(missing_payload)

        unknown_candidate = context_payload()
        candidate = dict(candidate_payload())
        candidate["discount"] = 10
        unknown_candidate["candidates"] = [candidate]
        with self.assertRaisesRegex(ContractError, "unknown field"):
            DecisionContext.from_dict(unknown_candidate)

    # test_versions_are_exact verifies incompatible schemas never fall back.
    def test_versions_are_exact(self) -> None:
        payload = context_payload()
        payload["schemaVersion"] = "agentpay.recommendation-context.v2"
        with self.assertRaisesRegex(ContractError, "schemaVersion"):
            DecisionContext.from_dict(payload)

        candidate = candidate_payload()
        candidate["schemaVersion"] = "agentpay.candidate-offer.v0"
        with self.assertRaisesRegex(ContractError, "schemaVersion"):
            CandidateOffer.from_dict(candidate)

    # test_atomic_amounts_never_accept_numeric_or_noncanonical_values verifies money safety.
    def test_atomic_amounts_never_accept_numeric_or_noncanonical_values(self) -> None:
        for invalid_amount in (125, 125.0, "0", "001", "-1", "1.5", "1e3"):
            with self.subTest(amount=invalid_amount):
                payload = candidate_payload()
                payload["amount"] = invalid_amount
                with self.assertRaisesRegex(ContractError, "amount"):
                    CandidateOffer.from_dict(payload)

    # test_sensitive_fields_are_rejected_recursively protects the training boundary.
    def test_sensitive_fields_are_rejected_recursively(self) -> None:
        for field_name in (
            "authorization",
            "paymentProof",
            "approvalToken",
            "walletPrivateKey",
            "signingSecret",
            "integrationCredential",
            "prompt",
            "requestBody",
            "responseBody",
        ):
            with self.subTest(field_name=field_name):
                payload = context_payload()
                payload["metadata"] = {field_name: "must-not-enter-rl"}
                with self.assertRaisesRegex(ContractError, "forbidden field"):
                    DecisionContext.from_dict(payload)

    # test_recommendation_rejects_nonfinite_or_mismatched_scores verifies output safety.
    def test_recommendation_rejects_nonfinite_or_mismatched_scores(self) -> None:
        payload = {
            "schemaVersion": RECOMMENDATION_SCHEMA_VERSION,
            "recommendationId": "rec_alpha",
            "requestId": "request_alpha",
            "rankedOfferIds": ["offer_alpha"],
            "scores": {"offer_beta": math.inf},
            "reasons": ["within budget"],
            "strategy": "deterministic-baseline",
            "modelVersion": "baseline-v1",
            "featureVersion": FEATURE_VERSION_V1,
        }
        with self.assertRaises(ContractError):
            Recommendation.from_dict(payload)

    # test_json_decoder_rejects_non_objects verifies the root wire type.
    def test_json_decoder_rejects_non_objects(self) -> None:
        with self.assertRaisesRegex(ContractError, "JSON object"):
            DecisionContext.from_json(json.dumps([context_payload()]))


if __name__ == "__main__":
    unittest.main()
