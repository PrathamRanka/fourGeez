"""Deterministic validation and feature construction for candidate offers."""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from enum import Enum

from .contracts import (
    FEATURE_VERSION_V1,
    MAXIMUM_BASIS_POINTS,
    MAXIMUM_LATENCY_MS,
    CandidateOffer,
    DecisionContext,
)

FEATURE_NAMES = (
    "price_weight",
    "quality_weight",
    "latency_weight",
    "price_score",
    "delivery_rate",
    "dispute_free_rate",
    "latency_score",
)
FEATURE_DIM = len(FEATURE_NAMES)


class RejectionReason(str, Enum):
    """Explains why a candidate never reached a ranking strategy."""

    UNAVAILABLE = "unavailable"
    INELIGIBLE = "ineligible"
    OVER_BUDGET = "over_budget"


@dataclass(frozen=True)
class CandidateRejection:
    """Records one deterministic pre-ranking rejection."""

    offer_id: str
    reason: RejectionReason


@dataclass(frozen=True)
class FeatureBatch:
    """Contains ordered candidate identifiers and versioned feature rows."""

    request_id: str
    feature_version: str
    feature_names: tuple[str, ...]
    candidate_ids: tuple[str, ...]
    rows: tuple[tuple[float, ...], ...]
    rejected: tuple[CandidateRejection, ...]


# build_feature_batch filters candidates and constructs stable normalized rows.
def build_feature_batch(context: DecisionContext) -> FeatureBatch:
    accepted: list[CandidateOffer] = []
    rejected: list[CandidateRejection] = []
    maximum_amount = int(context.maximum_amount)

    for offer in sorted(context.candidates, key=lambda candidate: candidate.offer_id):
        rejection_reason = _rejection_reason(offer, maximum_amount)
        if rejection_reason is not None:
            rejected.append(
                CandidateRejection(
                    offer_id=offer.offer_id,
                    reason=rejection_reason,
                )
            )
            continue
        accepted.append(offer)

    rows = tuple(_feature_row(context, offer) for offer in accepted)
    return FeatureBatch(
        request_id=context.request_id,
        feature_version=FEATURE_VERSION_V1,
        feature_names=FEATURE_NAMES,
        candidate_ids=tuple(offer.offer_id for offer in accepted),
        rows=rows,
        rejected=tuple(rejected),
    )


# _rejection_reason applies availability, eligibility, and budget gates in order.
def _rejection_reason(
    offer: CandidateOffer,
    maximum_amount: int,
) -> RejectionReason | None:
    if not offer.available:
        return RejectionReason.UNAVAILABLE
    if not offer.eligible:
        return RejectionReason.INELIGIBLE
    if int(offer.amount) > maximum_amount:
        return RejectionReason.OVER_BUDGET
    return None


# _feature_row converts one accepted offer into the fixed feature order.
def _feature_row(
    context: DecisionContext,
    offer: CandidateOffer,
) -> tuple[float, ...]:
    basis_points = Decimal(MAXIMUM_BASIS_POINTS)
    maximum_amount = Decimal(context.maximum_amount)
    price_score = Decimal(1) - (Decimal(offer.amount) / maximum_amount)
    latency_score = Decimal(1) - (Decimal(offer.p95_latency_ms) / Decimal(MAXIMUM_LATENCY_MS))
    return (
        float(Decimal(context.price_weight_bps) / basis_points),
        float(Decimal(context.quality_weight_bps) / basis_points),
        float(Decimal(context.latency_weight_bps) / basis_points),
        float(_clamp_unit_interval(price_score)),
        float(Decimal(offer.historical_delivery_rate_bps) / basis_points),
        float(Decimal(1) - (Decimal(offer.historical_dispute_rate_bps) / basis_points)),
        float(_clamp_unit_interval(latency_score)),
    )


# _clamp_unit_interval bounds normalized derived values without hiding missing data.
def _clamp_unit_interval(value: Decimal) -> Decimal:
    return max(Decimal(0), min(Decimal(1), value))
