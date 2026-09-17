"""Bounded JSON adapter for recommendation calls from the Go backend."""

from __future__ import annotations

import time
from collections.abc import Callable

from .contracts import ContractError, DecisionContext, Recommendation
from .features import build_feature_batch
from .ranking import Ranker

DEFAULT_MAXIMUM_REQUEST_BYTES = 64 * 1024
DEFAULT_MAXIMUM_RESPONSE_BYTES = 64 * 1024
DEFAULT_TIMEOUT_SECONDS = 0.25


class ServiceError(ValueError):
    """Reports a stable adapter error code without leaking sensitive input."""

    # __init__ stores a machine-readable code and safe message.
    def __init__(self, code: str, message: str) -> None:
        super().__init__(f"{code}: {message}")
        self.code = code


class RecommendationService:
    """Validates bounded JSON and revalidates every ranker response."""

    # __init__ configures explicit request, response, and time limits.
    def __init__(
        self,
        ranker: Ranker,
        maximum_request_bytes: int = DEFAULT_MAXIMUM_REQUEST_BYTES,
        maximum_response_bytes: int = DEFAULT_MAXIMUM_RESPONSE_BYTES,
        timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
        monotonic_clock: Callable[[], float] = time.monotonic,
    ) -> None:
        if maximum_request_bytes <= 0:
            raise ValueError("maximum_request_bytes must be positive")
        if maximum_response_bytes <= 0:
            raise ValueError("maximum_response_bytes must be positive")
        if timeout_seconds <= 0:
            raise ValueError("timeout_seconds must be positive")
        self._ranker = ranker
        self._maximum_request_bytes = maximum_request_bytes
        self._maximum_response_bytes = maximum_response_bytes
        self._timeout_seconds = timeout_seconds
        self._monotonic_clock = monotonic_clock

    # recommend parses, ranks, revalidates, and serializes one bounded request.
    def recommend(self, request_body: bytes) -> bytes:
        if len(request_body) > self._maximum_request_bytes:
            raise ServiceError("request_too_large", "request body exceeds configured limit")
        try:
            request_json = request_body.decode("utf-8")
        except UnicodeDecodeError:
            raise ServiceError("invalid_request", "request body must be UTF-8 JSON") from None
        try:
            context = DecisionContext.from_json(request_json)
        except ContractError as error:
            raise ServiceError("invalid_request", str(error)) from None

        started_at = self._monotonic_clock()
        feature_batch = build_feature_batch(context)
        recommendation = self._ranker.rank(context, feature_batch)
        elapsed_seconds = self._monotonic_clock() - started_at
        if elapsed_seconds > self._timeout_seconds:
            raise ServiceError("ranking_timeout", "ranking exceeded configured time limit")

        _validate_recommendation(
            context,
            feature_batch.candidate_ids,
            recommendation,
            self._ranker,
        )
        response_body = recommendation.to_json().encode("utf-8")
        if len(response_body) > self._maximum_response_bytes:
            raise ServiceError("response_too_large", "response exceeds configured limit")
        return response_body

    # health reports adapter availability without model or training contents.
    def health(self) -> dict[str, str]:
        return {"status": "ok"}

    # model_metadata exposes only identifiers required for compatibility checks.
    def model_metadata(self) -> dict[str, str]:
        return {
            "strategy": self._ranker.strategy,
            "modelVersion": self._ranker.model_version,
        }

    # maximum_request_bytes exposes the read limit to transport adapters.
    @property
    def maximum_request_bytes(self) -> int:
        return self._maximum_request_bytes


# _validate_recommendation rejects ranker output that escapes the input candidates.
def _validate_recommendation(
    context: DecisionContext,
    accepted_offer_ids: tuple[str, ...],
    recommendation: Recommendation,
    ranker: Ranker,
) -> None:
    if recommendation.request_id != context.request_id:
        raise ServiceError("invalid_recommendation", "request ID does not match")
    if recommendation.feature_version != context.feature_version:
        raise ServiceError("invalid_recommendation", "feature version does not match")
    if recommendation.strategy != ranker.strategy:
        raise ServiceError("invalid_recommendation", "strategy does not match ranker")
    if recommendation.model_version != ranker.model_version:
        raise ServiceError("invalid_recommendation", "model version does not match ranker")
    if set(recommendation.ranked_offer_ids) != set(accepted_offer_ids):
        raise ServiceError(
            "invalid_recommendation",
            "ranked offers do not exactly match validated candidates",
        )
