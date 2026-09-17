"""Strict versioned contracts for offline recommendation research."""

from __future__ import annotations

import json
import math
import re
from collections.abc import Mapping
from dataclasses import dataclass
from datetime import UTC, datetime
from enum import Enum
from typing import Any

DECISION_CONTEXT_SCHEMA_VERSION = "agentpay.recommendation-context.v1"
CANDIDATE_OFFER_SCHEMA_VERSION = "agentpay.candidate-offer.v1"
RECOMMENDATION_SCHEMA_VERSION = "agentpay.recommendation.v1"
OUTCOME_EVENT_SCHEMA_VERSION = "agentpay.recommendation-outcome.v1"
FEATURE_VERSION_V1 = "agentpay.features.v1"
MAXIMUM_CANDIDATES = 50
MAXIMUM_BASIS_POINTS = 10_000
MAXIMUM_LATENCY_MS = 300_000

_ATOMIC_AMOUNT_PATTERN = re.compile(r"^[1-9][0-9]*$")
_IDENTIFIER_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$")
_FORBIDDEN_FIELD_NAMES = {
    "approvaltoken",
    "authorization",
    "cookie",
    "integrationcredential",
    "paymentproof",
    "paymentsignature",
    "prompt",
    "requestbody",
    "responsebody",
    "signingsecret",
    "walletkey",
    "walletprivatekey",
}


class ContractError(ValueError):
    """Reports one strict recommendation-contract violation."""


class OutcomeType(str, Enum):
    """Identifies one allowlisted offline recommendation outcome."""

    RECOMMENDATION_ACCEPTED = "recommendation.accepted"
    RECOMMENDATION_OVERRIDDEN = "recommendation.overridden"
    TRANSACTION_FULFILLED = "transaction.fulfilled"
    TRANSACTION_FAILED = "transaction.failed"
    TRANSACTION_DISPUTED = "transaction.disputed"
    DISPUTE_RESOLVED = "dispute.resolved"


@dataclass(frozen=True)
class CandidateOffer:
    """Contains one already-normalized seller offer presented for ranking."""

    schema_version: str
    offer_id: str
    seller_id: str
    amount: str
    asset: str
    network: str
    capabilities: tuple[str, ...]
    available: bool
    eligible: bool
    historical_delivery_rate_bps: int
    historical_dispute_rate_bps: int
    p95_latency_ms: int

    # __post_init__ validates direct Python construction as strictly as JSON.
    def __post_init__(self) -> None:
        _require_version("schemaVersion", self.schema_version, CANDIDATE_OFFER_SCHEMA_VERSION)
        _require_identifier("offerId", self.offer_id)
        _require_identifier("sellerId", self.seller_id)
        _require_atomic_amount("amount", self.amount)
        _require_text("asset", self.asset, 160)
        _require_text("network", self.network, 80)
        _require_capabilities(self.capabilities)
        _require_bool("available", self.available)
        _require_bool("eligible", self.eligible)
        _require_basis_points(
            "historicalDeliveryRateBps",
            self.historical_delivery_rate_bps,
        )
        _require_basis_points(
            "historicalDisputeRateBps",
            self.historical_dispute_rate_bps,
        )
        _require_integer_range("p95LatencyMs", self.p95_latency_ms, 1, MAXIMUM_LATENCY_MS)

    # from_dict parses the exact camel-case candidate wire shape.
    @classmethod
    def from_dict(cls, payload: Mapping[str, Any]) -> CandidateOffer:
        _reject_sensitive_fields(payload)
        _require_exact_fields(
            payload,
            required={
                "schemaVersion",
                "offerId",
                "sellerId",
                "amount",
                "asset",
                "network",
                "capabilities",
                "available",
                "eligible",
                "historicalDeliveryRateBps",
                "historicalDisputeRateBps",
                "p95LatencyMs",
            },
        )
        capabilities = payload["capabilities"]
        if not isinstance(capabilities, list):
            raise ContractError("capabilities must be a JSON array")
        return cls(
            schema_version=payload["schemaVersion"],
            offer_id=payload["offerId"],
            seller_id=payload["sellerId"],
            amount=payload["amount"],
            asset=payload["asset"],
            network=payload["network"],
            capabilities=tuple(capabilities),
            available=payload["available"],
            eligible=payload["eligible"],
            historical_delivery_rate_bps=payload["historicalDeliveryRateBps"],
            historical_dispute_rate_bps=payload["historicalDisputeRateBps"],
            p95_latency_ms=payload["p95LatencyMs"],
        )

    # from_json decodes one strict candidate-offer JSON object.
    @classmethod
    def from_json(cls, payload: str) -> CandidateOffer:
        return cls.from_dict(_decode_json_object(payload))

    # to_dict serializes the stable candidate wire shape.
    def to_dict(self) -> dict[str, Any]:
        return {
            "schemaVersion": self.schema_version,
            "offerId": self.offer_id,
            "sellerId": self.seller_id,
            "amount": self.amount,
            "asset": self.asset,
            "network": self.network,
            "capabilities": list(self.capabilities),
            "available": self.available,
            "eligible": self.eligible,
            "historicalDeliveryRateBps": self.historical_delivery_rate_bps,
            "historicalDisputeRateBps": self.historical_dispute_rate_bps,
            "p95LatencyMs": self.p95_latency_ms,
        }

    # to_json serializes canonical compact JSON for fixtures and adapters.
    def to_json(self) -> str:
        return json.dumps(self.to_dict(), sort_keys=True, separators=(",", ":"))


@dataclass(frozen=True)
class DecisionContext:
    """Contains one bounded ranking request and its candidate set."""

    schema_version: str
    request_id: str
    feature_version: str
    maximum_amount: str
    price_weight_bps: int
    quality_weight_bps: int
    latency_weight_bps: int
    candidates: tuple[CandidateOffer, ...]
    buyer_segment_id: str | None = None

    # __post_init__ validates direct Python construction as strictly as JSON.
    def __post_init__(self) -> None:
        _require_version("schemaVersion", self.schema_version, DECISION_CONTEXT_SCHEMA_VERSION)
        _require_identifier("requestId", self.request_id)
        _require_version("featureVersion", self.feature_version, FEATURE_VERSION_V1)
        _require_atomic_amount("maximumAmount", self.maximum_amount)
        _require_basis_points("priceWeightBps", self.price_weight_bps)
        _require_basis_points("qualityWeightBps", self.quality_weight_bps)
        _require_basis_points("latencyWeightBps", self.latency_weight_bps)
        if self.price_weight_bps + self.quality_weight_bps + self.latency_weight_bps <= 0:
            raise ContractError("preference weights must have a positive total")
        if self.buyer_segment_id is not None:
            _require_identifier("buyerSegmentId", self.buyer_segment_id)
        if not 1 <= len(self.candidates) <= MAXIMUM_CANDIDATES:
            raise ContractError("candidates must contain 1-50 offers")
        offer_ids = [candidate.offer_id for candidate in self.candidates]
        if len(set(offer_ids)) != len(offer_ids):
            raise ContractError("candidate offerId values must be unique")

    # from_dict parses the exact camel-case decision-context wire shape.
    @classmethod
    def from_dict(cls, payload: Mapping[str, Any]) -> DecisionContext:
        _reject_sensitive_fields(payload)
        _require_exact_fields(
            payload,
            required={
                "schemaVersion",
                "requestId",
                "featureVersion",
                "maximumAmount",
                "priceWeightBps",
                "qualityWeightBps",
                "latencyWeightBps",
                "candidates",
            },
            optional={"buyerSegmentId"},
        )
        candidate_payloads = payload["candidates"]
        if not isinstance(candidate_payloads, list):
            raise ContractError("candidates must be a JSON array")
        return cls(
            schema_version=payload["schemaVersion"],
            request_id=payload["requestId"],
            feature_version=payload["featureVersion"],
            buyer_segment_id=payload.get("buyerSegmentId"),
            maximum_amount=payload["maximumAmount"],
            price_weight_bps=payload["priceWeightBps"],
            quality_weight_bps=payload["qualityWeightBps"],
            latency_weight_bps=payload["latencyWeightBps"],
            candidates=tuple(
                CandidateOffer.from_dict(candidate) for candidate in candidate_payloads
            ),
        )

    # from_json decodes one strict decision-context JSON object.
    @classmethod
    def from_json(cls, payload: str) -> DecisionContext:
        return cls.from_dict(_decode_json_object(payload))

    # to_dict serializes the stable decision-context wire shape.
    def to_dict(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "schemaVersion": self.schema_version,
            "requestId": self.request_id,
            "featureVersion": self.feature_version,
            "maximumAmount": self.maximum_amount,
            "priceWeightBps": self.price_weight_bps,
            "qualityWeightBps": self.quality_weight_bps,
            "latencyWeightBps": self.latency_weight_bps,
            "candidates": [candidate.to_dict() for candidate in self.candidates],
        }
        if self.buyer_segment_id is not None:
            payload["buyerSegmentId"] = self.buyer_segment_id
        return payload

    # to_json serializes canonical compact JSON for fixtures and adapters.
    def to_json(self) -> str:
        return json.dumps(self.to_dict(), sort_keys=True, separators=(",", ":"))


@dataclass(frozen=True)
class Recommendation:
    """Contains a non-authoritative ranked suggestion and version metadata."""

    schema_version: str
    recommendation_id: str
    request_id: str
    ranked_offer_ids: tuple[str, ...]
    scores: Mapping[str, float]
    reasons: tuple[str, ...]
    strategy: str
    model_version: str
    feature_version: str

    # __post_init__ validates direct Python construction as strictly as JSON.
    def __post_init__(self) -> None:
        _require_version("schemaVersion", self.schema_version, RECOMMENDATION_SCHEMA_VERSION)
        _require_identifier("recommendationId", self.recommendation_id)
        _require_identifier("requestId", self.request_id)
        _require_version("featureVersion", self.feature_version, FEATURE_VERSION_V1)
        _require_text("strategy", self.strategy, 120)
        _require_text("modelVersion", self.model_version, 120)
        if not self.ranked_offer_ids:
            raise ContractError("rankedOfferIds must not be empty")
        for offer_id in self.ranked_offer_ids:
            _require_identifier("rankedOfferIds", offer_id)
        if len(set(self.ranked_offer_ids)) != len(self.ranked_offer_ids):
            raise ContractError("rankedOfferIds must be unique")
        if set(self.scores) != set(self.ranked_offer_ids):
            raise ContractError("scores must exactly match rankedOfferIds")
        for offer_id, score in self.scores.items():
            _require_identifier("scores offerId", offer_id)
            if isinstance(score, bool) or not isinstance(score, (int, float)):
                raise ContractError("scores must contain numeric values")
            if not math.isfinite(float(score)):
                raise ContractError("scores must contain finite values")
        if not self.reasons:
            raise ContractError("reasons must not be empty")
        for reason in self.reasons:
            _require_text("reasons", reason, 500)

    # from_dict parses the exact camel-case recommendation wire shape.
    @classmethod
    def from_dict(cls, payload: Mapping[str, Any]) -> Recommendation:
        _reject_sensitive_fields(payload)
        _require_exact_fields(
            payload,
            required={
                "schemaVersion",
                "recommendationId",
                "requestId",
                "rankedOfferIds",
                "scores",
                "reasons",
                "strategy",
                "modelVersion",
                "featureVersion",
            },
        )
        ranked_offer_ids = payload["rankedOfferIds"]
        reasons = payload["reasons"]
        scores = payload["scores"]
        if not isinstance(ranked_offer_ids, list):
            raise ContractError("rankedOfferIds must be a JSON array")
        if not isinstance(reasons, list):
            raise ContractError("reasons must be a JSON array")
        if not isinstance(scores, Mapping):
            raise ContractError("scores must be a JSON object")
        converted_scores: dict[str, float] = {}
        for offer_id, score in scores.items():
            if isinstance(score, bool) or not isinstance(score, (int, float)):
                raise ContractError("scores must contain numeric values")
            converted_scores[str(offer_id)] = float(score)
        return cls(
            schema_version=payload["schemaVersion"],
            recommendation_id=payload["recommendationId"],
            request_id=payload["requestId"],
            ranked_offer_ids=tuple(ranked_offer_ids),
            scores=converted_scores,
            reasons=tuple(reasons),
            strategy=payload["strategy"],
            model_version=payload["modelVersion"],
            feature_version=payload["featureVersion"],
        )

    # from_json decodes one strict recommendation JSON object.
    @classmethod
    def from_json(cls, payload: str) -> Recommendation:
        return cls.from_dict(_decode_json_object(payload))

    # to_dict serializes the stable recommendation wire shape.
    def to_dict(self) -> dict[str, Any]:
        return {
            "schemaVersion": self.schema_version,
            "recommendationId": self.recommendation_id,
            "requestId": self.request_id,
            "rankedOfferIds": list(self.ranked_offer_ids),
            "scores": dict(self.scores),
            "reasons": list(self.reasons),
            "strategy": self.strategy,
            "modelVersion": self.model_version,
            "featureVersion": self.feature_version,
        }

    # to_json serializes canonical compact JSON for fixtures and adapters.
    def to_json(self) -> str:
        return json.dumps(self.to_dict(), sort_keys=True, separators=(",", ":"))


@dataclass(frozen=True)
class OutcomeEvent:
    """Contains one allowlisted, versioned offline learning event."""

    schema_version: str
    event_id: str
    recommendation_id: str
    offer_id: str
    event_type: OutcomeType
    occurred_at: str
    feature_version: str
    strategy_version: str
    model_version: str

    # __post_init__ validates direct Python construction as strictly as JSON.
    def __post_init__(self) -> None:
        _require_version("schemaVersion", self.schema_version, OUTCOME_EVENT_SCHEMA_VERSION)
        _require_identifier("eventId", self.event_id)
        _require_identifier("recommendationId", self.recommendation_id)
        _require_identifier("offerId", self.offer_id)
        if not isinstance(self.event_type, OutcomeType):
            raise ContractError("eventType is unsupported")
        _require_utc_timestamp("occurredAt", self.occurred_at)
        _require_version("featureVersion", self.feature_version, FEATURE_VERSION_V1)
        _require_text("strategyVersion", self.strategy_version, 120)
        _require_text("modelVersion", self.model_version, 120)

    # from_dict parses the exact camel-case outcome-event wire shape.
    @classmethod
    def from_dict(cls, payload: Mapping[str, Any]) -> OutcomeEvent:
        _reject_sensitive_fields(payload)
        _require_exact_fields(
            payload,
            required={
                "schemaVersion",
                "eventId",
                "recommendationId",
                "offerId",
                "eventType",
                "occurredAt",
                "featureVersion",
                "strategyVersion",
                "modelVersion",
            },
        )
        try:
            event_type = OutcomeType(payload["eventType"])
        except (TypeError, ValueError):
            raise ContractError("eventType is unsupported") from None
        return cls(
            schema_version=payload["schemaVersion"],
            event_id=payload["eventId"],
            recommendation_id=payload["recommendationId"],
            offer_id=payload["offerId"],
            event_type=event_type,
            occurred_at=payload["occurredAt"],
            feature_version=payload["featureVersion"],
            strategy_version=payload["strategyVersion"],
            model_version=payload["modelVersion"],
        )

    # from_json decodes one strict outcome-event JSON object.
    @classmethod
    def from_json(cls, payload: str) -> OutcomeEvent:
        return cls.from_dict(_decode_json_object(payload))

    # to_dict serializes the stable outcome-event wire shape.
    def to_dict(self) -> dict[str, Any]:
        return {
            "schemaVersion": self.schema_version,
            "eventId": self.event_id,
            "recommendationId": self.recommendation_id,
            "offerId": self.offer_id,
            "eventType": self.event_type.value,
            "occurredAt": self.occurred_at,
            "featureVersion": self.feature_version,
            "strategyVersion": self.strategy_version,
            "modelVersion": self.model_version,
        }

    # to_json serializes canonical compact JSON for fixtures and adapters.
    def to_json(self) -> str:
        return json.dumps(self.to_dict(), sort_keys=True, separators=(",", ":"))


# _decode_json_object accepts only a JSON object at the contract root.
def _decode_json_object(payload: str) -> Mapping[str, Any]:
    try:
        decoded = json.loads(payload)
    except (TypeError, json.JSONDecodeError):
        raise ContractError("payload must be valid JSON") from None
    if not isinstance(decoded, Mapping):
        raise ContractError("payload must be a JSON object")
    return decoded


# _require_exact_fields rejects absent and undocumented keys.
def _require_exact_fields(
    payload: Mapping[str, Any],
    required: set[str],
    optional: set[str] | None = None,
) -> None:
    if not isinstance(payload, Mapping):
        raise ContractError("payload must be a JSON object")
    allowed = required | (optional or set())
    missing = sorted(required - set(payload))
    if missing:
        raise ContractError(f"missing field: {missing[0]}")
    unknown = sorted(set(payload) - allowed)
    if unknown:
        raise ContractError(f"unknown field: {unknown[0]}")


# _reject_sensitive_fields blocks secrets and unrestricted payloads recursively.
def _reject_sensitive_fields(value: Any, path: str = "$") -> None:
    if isinstance(value, Mapping):
        for field_name, nested_value in value.items():
            normalized_name = re.sub(r"[^a-z0-9]", "", str(field_name).lower())
            if normalized_name in _FORBIDDEN_FIELD_NAMES:
                raise ContractError(f"forbidden field: {path}.{field_name}")
            _reject_sensitive_fields(nested_value, f"{path}.{field_name}")
    elif isinstance(value, list):
        for index, nested_value in enumerate(value):
            _reject_sensitive_fields(nested_value, f"{path}[{index}]")


# _require_version enforces exact schema and feature compatibility.
def _require_version(field_name: str, value: Any, expected: str) -> None:
    if value != expected:
        raise ContractError(f"{field_name} must equal {expected}")


# _require_identifier validates bounded opaque identifiers without interpreting them.
def _require_identifier(field_name: str, value: Any) -> None:
    if not isinstance(value, str) or _IDENTIFIER_PATTERN.fullmatch(value) is None:
        raise ContractError(f"{field_name} must be a bounded opaque identifier")


# _require_atomic_amount validates canonical positive base-10 money strings.
def _require_atomic_amount(field_name: str, value: Any) -> None:
    if not isinstance(value, str) or _ATOMIC_AMOUNT_PATTERN.fullmatch(value) is None:
        raise ContractError(f"{field_name} must be a positive atomic-unit string")


# _require_text validates bounded non-empty display or version text.
def _require_text(field_name: str, value: Any, maximum_length: int) -> None:
    if not isinstance(value, str) or not value.strip() or len(value) > maximum_length:
        raise ContractError(f"{field_name} must contain 1-{maximum_length} characters")


# _require_bool rejects integers that Python otherwise treats as booleans.
def _require_bool(field_name: str, value: Any) -> None:
    if not isinstance(value, bool):
        raise ContractError(f"{field_name} must be a boolean")


# _require_integer_range validates a non-boolean bounded integer.
def _require_integer_range(field_name: str, value: Any, minimum: int, maximum: int) -> None:
    if isinstance(value, bool) or not isinstance(value, int) or not minimum <= value <= maximum:
        raise ContractError(f"{field_name} must be an integer from {minimum} to {maximum}")


# _require_basis_points validates a normalized integer rate or preference weight.
def _require_basis_points(field_name: str, value: Any) -> None:
    _require_integer_range(field_name, value, 0, MAXIMUM_BASIS_POINTS)


# _require_capabilities validates a bounded unique capability list.
def _require_capabilities(capabilities: tuple[str, ...]) -> None:
    if not isinstance(capabilities, tuple) or not 1 <= len(capabilities) <= 32:
        raise ContractError("capabilities must contain 1-32 values")
    for capability in capabilities:
        _require_text("capabilities", capability, 80)
    if len(set(capabilities)) != len(capabilities):
        raise ContractError("capabilities must be unique")


# _require_utc_timestamp validates an RFC 3339 timestamp using the UTC designator.
def _require_utc_timestamp(field_name: str, value: Any) -> None:
    if not isinstance(value, str) or not value.endswith("Z"):
        raise ContractError(f"{field_name} must be an RFC 3339 UTC timestamp")
    try:
        timestamp = datetime.fromisoformat(value.removesuffix("Z") + "+00:00")
    except ValueError:
        raise ContractError(f"{field_name} must be an RFC 3339 UTC timestamp") from None
    if timestamp.utcoffset() != UTC.utcoffset(timestamp):
        raise ContractError(f"{field_name} must be an RFC 3339 UTC timestamp")
