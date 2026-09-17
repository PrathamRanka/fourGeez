"""Reproducible synthetic recommendation scenarios for offline research."""

from __future__ import annotations

import hashlib
import json
import random
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import Any

from .baseline import MODEL_VERSION, STRATEGY_NAME, DeterministicBaseline
from .contracts import (
    CANDIDATE_OFFER_SCHEMA_VERSION,
    DECISION_CONTEXT_SCHEMA_VERSION,
    FEATURE_VERSION_V1,
    OUTCOME_EVENT_SCHEMA_VERSION,
    CandidateOffer,
    DecisionContext,
    OutcomeEvent,
    OutcomeType,
    Recommendation,
)
from .features import build_feature_batch

DATASET_SCHEMA_VERSION = "agentpay.synthetic-dataset.v1"
MAXIMUM_SYNTHETIC_SCENARIOS = 10_000
SYNTHETIC_SEGMENTS = (
    "cost-sensitive",
    "quality-sensitive",
    "latency-sensitive",
)
_BASE_TIMESTAMP = datetime(2026, 1, 1, tzinfo=UTC)


@dataclass(frozen=True)
class SyntheticScenario:
    """Groups one ranking input with outcomes generated only after ranking."""

    segment: str
    context: DecisionContext
    recommendation: Recommendation
    outcome_events: tuple[OutcomeEvent, ...]

    # to_dict serializes one scenario without adding hidden training features.
    def to_dict(self) -> dict[str, Any]:
        return {
            "segment": self.segment,
            "context": self.context.to_dict(),
            "recommendation": self.recommendation.to_dict(),
            "outcomeEvents": [event.to_dict() for event in self.outcome_events],
        }

    # from_dict validates one serialized synthetic scenario.
    @classmethod
    def from_dict(cls, scenario_payload: object) -> SyntheticScenario:
        if not isinstance(scenario_payload, dict):
            raise TypeError("synthetic scenario must be a JSON object")
        required_fields = {"segment", "context", "recommendation", "outcomeEvents"}
        if set(scenario_payload) != required_fields:
            raise ValueError("synthetic scenario fields are incompatible")
        segment = scenario_payload["segment"]
        if segment not in SYNTHETIC_SEGMENTS:
            raise ValueError("synthetic scenario segment is unsupported")
        context_payload = scenario_payload["context"]
        recommendation_payload = scenario_payload["recommendation"]
        outcome_payloads = scenario_payload["outcomeEvents"]
        if not isinstance(context_payload, dict):
            raise TypeError("synthetic context must be a JSON object")
        if not isinstance(recommendation_payload, dict):
            raise TypeError("synthetic recommendation must be a JSON object")
        if not isinstance(outcome_payloads, list):
            raise TypeError("synthetic outcomeEvents must be a JSON array")
        context = DecisionContext.from_dict(context_payload)
        recommendation = Recommendation.from_dict(recommendation_payload)
        outcome_events = tuple(
            OutcomeEvent.from_dict(outcome_payload)
            for outcome_payload in outcome_payloads
        )
        _validate_scenario_references(context, recommendation, outcome_events)
        return cls(
            segment=segment,
            context=context,
            recommendation=recommendation,
            outcome_events=outcome_events,
        )


@dataclass(frozen=True)
class SyntheticDataset:
    """Contains a bounded dataset with explicit synthetic provenance."""

    schema_version: str
    synthetic: bool
    seed: int
    scenarios: tuple[SyntheticScenario, ...]

    # to_dict serializes stable metadata and scenario ordering.
    def to_dict(self) -> dict[str, Any]:
        return {
            "schemaVersion": self.schema_version,
            "synthetic": self.synthetic,
            "seed": self.seed,
            "scenarios": [scenario.to_dict() for scenario in self.scenarios],
        }

    # to_json returns canonical JSON suitable for fingerprints and local files.
    def to_json(self) -> str:
        return json.dumps(self.to_dict(), sort_keys=True, separators=(",", ":"))

    # from_json validates a complete serialized synthetic dataset.
    @classmethod
    def from_json(cls, serialized_dataset: str) -> SyntheticDataset:
        try:
            dataset_payload = json.loads(serialized_dataset)
        except json.JSONDecodeError:
            raise ValueError("synthetic dataset is not valid JSON") from None
        if not isinstance(dataset_payload, dict):
            raise TypeError("synthetic dataset must be a JSON object")
        required_fields = {"schemaVersion", "synthetic", "seed", "scenarios"}
        if set(dataset_payload) != required_fields:
            raise ValueError("synthetic dataset fields are incompatible")
        if dataset_payload["schemaVersion"] != DATASET_SCHEMA_VERSION:
            raise ValueError("synthetic dataset version is unsupported")
        if dataset_payload["synthetic"] is not True:
            raise ValueError("synthetic dataset must be labeled synthetic")
        seed = dataset_payload["seed"]
        if isinstance(seed, bool) or not isinstance(seed, int):
            raise TypeError("synthetic dataset seed must be an integer")
        scenario_payloads = dataset_payload["scenarios"]
        if not isinstance(scenario_payloads, list):
            raise TypeError("synthetic scenarios must be a JSON array")
        if not 1 <= len(scenario_payloads) <= MAXIMUM_SYNTHETIC_SCENARIOS:
            raise ValueError("synthetic scenario count is outside the supported range")
        return cls(
            schema_version=DATASET_SCHEMA_VERSION,
            synthetic=True,
            seed=seed,
            scenarios=tuple(
                SyntheticScenario.from_dict(scenario_payload)
                for scenario_payload in scenario_payloads
            ),
        )

    # fingerprint identifies the exact synthetic dataset contents.
    @property
    def fingerprint(self) -> str:
        digest = hashlib.sha256(self.to_json().encode("utf-8")).hexdigest()
        return digest


# generate_synthetic_dataset creates deterministic scenarios from a local RNG.
def generate_synthetic_dataset(seed: int, scenario_count: int) -> SyntheticDataset:
    if not 1 <= scenario_count <= MAXIMUM_SYNTHETIC_SCENARIOS:
        raise ValueError(
            f"scenario_count must be between 1 and {MAXIMUM_SYNTHETIC_SCENARIOS}"
        )

    random_source = random.Random(seed)
    segment_offset = random_source.randrange(len(SYNTHETIC_SEGMENTS))
    scenarios = tuple(
        _generate_scenario(
            random_source=random_source,
            scenario_index=scenario_index,
            segment=SYNTHETIC_SEGMENTS[
                (scenario_index + segment_offset) % len(SYNTHETIC_SEGMENTS)
            ],
        )
        for scenario_index in range(scenario_count)
    )
    return SyntheticDataset(
        schema_version=DATASET_SCHEMA_VERSION,
        synthetic=True,
        seed=seed,
        scenarios=scenarios,
    )


# _generate_scenario creates candidates, ranks them, and only then samples outcomes.
def _generate_scenario(
    random_source: random.Random,
    scenario_index: int,
    segment: str,
) -> SyntheticScenario:
    context = _generate_context(random_source, scenario_index, segment)
    recommendation = DeterministicBaseline().rank(
        context,
        build_feature_batch(context),
    )
    outcome_events = _generate_outcomes(
        random_source,
        scenario_index,
        context,
        recommendation,
    )
    return SyntheticScenario(
        segment=segment,
        context=context,
        recommendation=recommendation,
        outcome_events=outcome_events,
    )


# _generate_context produces only facts available before a recommendation.
def _generate_context(
    random_source: random.Random,
    scenario_index: int,
    segment: str,
) -> DecisionContext:
    weights_by_segment = {
        "cost-sensitive": (6500, 2200, 1300),
        "quality-sensitive": (1800, 6800, 1400),
        "latency-sensitive": (1800, 1800, 6400),
    }
    price_weight, quality_weight, latency_weight = weights_by_segment[segment]
    maximum_amount = random_source.randint(700, 2000)
    candidates = tuple(
        _generate_candidate(
            random_source,
            scenario_index,
            candidate_index,
            maximum_amount,
        )
        for candidate_index in range(3)
    )
    return DecisionContext(
        schema_version=DECISION_CONTEXT_SCHEMA_VERSION,
        request_id=f"request_synthetic_{scenario_index:05d}",
        feature_version=FEATURE_VERSION_V1,
        buyer_segment_id=f"segment_{segment}",
        maximum_amount=str(maximum_amount),
        price_weight_bps=price_weight,
        quality_weight_bps=quality_weight,
        latency_weight_bps=latency_weight,
        candidates=candidates,
    )


# _generate_candidate samples bounded, valid pre-decision offer facts.
def _generate_candidate(
    random_source: random.Random,
    scenario_index: int,
    candidate_index: int,
    maximum_amount: int,
) -> CandidateOffer:
    amount = random_source.randint(max(1, maximum_amount // 5), maximum_amount)
    return CandidateOffer(
        schema_version=CANDIDATE_OFFER_SCHEMA_VERSION,
        offer_id=f"offer_synthetic_{scenario_index:05d}_{candidate_index}",
        seller_id=f"seller_synthetic_{candidate_index}",
        amount=str(amount),
        asset="USDC",
        network="eip155:84532",
        capabilities=("synthetic-research",),
        available=True,
        eligible=True,
        historical_delivery_rate_bps=random_source.randint(8000, 10_000),
        historical_dispute_rate_bps=random_source.randint(0, 1200),
        p95_latency_ms=random_source.randint(100, 30_000),
    )


# _generate_outcomes samples post-recommendation events without feature leakage.
def _generate_outcomes(
    random_source: random.Random,
    scenario_index: int,
    context: DecisionContext,
    recommendation: Recommendation,
) -> tuple[OutcomeEvent, ...]:
    selected_offer_id = recommendation.ranked_offer_ids[0]
    first_event_type = OutcomeType.RECOMMENDATION_ACCEPTED
    if len(recommendation.ranked_offer_ids) > 1 and random_source.random() < 0.12:
        selected_offer_id = recommendation.ranked_offer_ids[1]
        first_event_type = OutcomeType.RECOMMENDATION_OVERRIDDEN

    selected_offer = next(
        candidate
        for candidate in context.candidates
        if candidate.offer_id == selected_offer_id
    )
    delivery_probability = selected_offer.historical_delivery_rate_bps / 10_000
    delivered = random_source.random() < delivery_probability
    event_types = [first_event_type]
    if delivered:
        event_types.append(OutcomeType.TRANSACTION_FULFILLED)
        dispute_probability = selected_offer.historical_dispute_rate_bps / 10_000
        if random_source.random() < dispute_probability:
            event_types.append(OutcomeType.TRANSACTION_DISPUTED)
            event_types.append(OutcomeType.DISPUTE_RESOLVED)
    else:
        event_types.append(OutcomeType.TRANSACTION_FAILED)

    return tuple(
        _outcome_event(
            scenario_index,
            event_index,
            event_type,
            recommendation,
            selected_offer_id,
        )
        for event_index, event_type in enumerate(event_types)
    )


# _outcome_event stamps deterministic IDs and UTC timestamps on one event.
def _outcome_event(
    scenario_index: int,
    event_index: int,
    event_type: OutcomeType,
    recommendation: Recommendation,
    offer_id: str,
) -> OutcomeEvent:
    occurred_at = _BASE_TIMESTAMP + timedelta(
        minutes=scenario_index,
        seconds=event_index,
    )
    return OutcomeEvent(
        schema_version=OUTCOME_EVENT_SCHEMA_VERSION,
        event_id=f"event_synthetic_{scenario_index:05d}_{event_index}",
        recommendation_id=recommendation.recommendation_id,
        offer_id=offer_id,
        event_type=event_type,
        occurred_at=occurred_at.isoformat().replace("+00:00", "Z"),
        feature_version=recommendation.feature_version,
        strategy_version=STRATEGY_NAME,
        model_version=MODEL_VERSION,
    )


# _validate_scenario_references checks serialized recommendation and outcome links.
def _validate_scenario_references(
    context: DecisionContext,
    recommendation: Recommendation,
    outcome_events: tuple[OutcomeEvent, ...],
) -> None:
    if recommendation.request_id != context.request_id:
        raise ValueError("synthetic recommendation request does not match context")
    context_offer_ids = {candidate.offer_id for candidate in context.candidates}
    if not set(recommendation.ranked_offer_ids).issubset(context_offer_ids):
        raise ValueError("synthetic recommendation contains an unknown offer")
    for outcome_event in outcome_events:
        if outcome_event.recommendation_id != recommendation.recommendation_id:
            raise ValueError("synthetic outcome recommendation does not match")
        if outcome_event.offer_id not in recommendation.ranked_offer_ids:
            raise ValueError("synthetic outcome contains an unknown offer")
