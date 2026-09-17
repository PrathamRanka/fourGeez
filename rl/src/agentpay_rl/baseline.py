"""Deterministic, interpretable offer-ranking baseline."""

from __future__ import annotations

import hashlib
import json

from .contracts import RECOMMENDATION_SCHEMA_VERSION, DecisionContext, Recommendation
from .features import FEATURE_NAMES, FeatureBatch

STRATEGY_NAME = "deterministic-baseline"
MODEL_VERSION = "baseline-v1"


class NoRankableCandidates(ValueError):
    """Reports that every candidate was rejected before ranking."""


class FeatureBatchMismatch(ValueError):
    """Reports a feature batch that does not match its decision context."""


class DeterministicBaseline:
    """Ranks accepted offers with the fixed version-one scoring formula."""

    strategy = STRATEGY_NAME
    model_version = MODEL_VERSION

    # rank scores candidates without randomness or mutable model state.
    def rank(
        self,
        context: DecisionContext,
        batch: FeatureBatch,
    ) -> Recommendation:
        _validate_batch(context, batch)
        if not batch.candidate_ids:
            raise NoRankableCandidates("no candidate survived feature validation")

        score_by_offer: dict[str, float] = {}
        component_by_offer: dict[str, tuple[float, float, float]] = {}
        for offer_id, row in zip(batch.candidate_ids, batch.rows, strict=True):
            price_fit, quality_fit, latency_fit = _score_components(row)
            total_score = round(price_fit + quality_fit + latency_fit, 12)
            score_by_offer[offer_id] = total_score
            component_by_offer[offer_id] = (
                price_fit,
                quality_fit,
                latency_fit,
            )

        ranked_offer_ids = tuple(
            sorted(
                score_by_offer,
                key=lambda offer_id: (-score_by_offer[offer_id], offer_id),
            )
        )
        reasons = _winner_reasons(
            ranked_offer_ids[0],
            component_by_offer[ranked_offer_ids[0]],
        )
        return Recommendation(
            schema_version=RECOMMENDATION_SCHEMA_VERSION,
            recommendation_id=_recommendation_id(context, ranked_offer_ids, score_by_offer),
            request_id=context.request_id,
            ranked_offer_ids=ranked_offer_ids,
            scores={offer_id: score_by_offer[offer_id] for offer_id in ranked_offer_ids},
            reasons=reasons,
            strategy=self.strategy,
            model_version=self.model_version,
            feature_version=batch.feature_version,
        )


# _validate_batch rejects mismatched or malformed feature inputs.
def _validate_batch(context: DecisionContext, batch: FeatureBatch) -> None:
    if batch.request_id != context.request_id:
        raise FeatureBatchMismatch("feature batch requestId does not match context")
    if batch.feature_version != context.feature_version:
        raise FeatureBatchMismatch("feature batch version does not match context")
    if batch.feature_names != FEATURE_NAMES:
        raise FeatureBatchMismatch("feature columns do not match agentpay.features.v1")
    if len(batch.candidate_ids) != len(batch.rows):
        raise FeatureBatchMismatch("feature row count does not match candidate identifiers")
    for row in batch.rows:
        if len(row) != len(FEATURE_NAMES):
            raise FeatureBatchMismatch("feature row width does not match feature columns")


# _score_components applies the published baseline formula to one feature row.
def _score_components(row: tuple[float, ...]) -> tuple[float, float, float]:
    column = {name: index for index, name in enumerate(FEATURE_NAMES)}
    price_fit = row[column["price_weight"]] * row[column["price_score"]]
    quality_value = (row[column["delivery_rate"]] + row[column["dispute_free_rate"]]) / 2
    quality_fit = row[column["quality_weight"]] * quality_value
    latency_fit = row[column["latency_weight"]] * row[column["latency_score"]]
    return price_fit, quality_fit, latency_fit


# _winner_reasons explains the strongest components for the top-ranked offer.
def _winner_reasons(
    offer_id: str,
    components: tuple[float, float, float],
) -> tuple[str, ...]:
    named_components = (
        ("price fit", components[0]),
        ("quality fit", components[1]),
        ("latency fit", components[2]),
    )
    ordered = sorted(
        named_components,
        key=lambda component: (-component[1], component[0]),
    )
    return tuple(
        f"{offer_id} ranks strongly on {component_name}" for component_name, _ in ordered[:2]
    )


# _recommendation_id derives a stable identifier from the complete ranking.
def _recommendation_id(
    context: DecisionContext,
    ranked_offer_ids: tuple[str, ...],
    score_by_offer: dict[str, float],
) -> str:
    canonical = json.dumps(
        {
            "featureVersion": context.feature_version,
            "rankedOfferIds": ranked_offer_ids,
            "requestId": context.request_id,
            "scores": {
                offer_id: format(score_by_offer[offer_id], ".12f") for offer_id in ranked_offer_ids
            },
        },
        sort_keys=True,
        separators=(",", ":"),
    )
    digest = hashlib.sha256(canonical.encode("utf-8")).hexdigest()
    return "rec_" + digest[:26]
