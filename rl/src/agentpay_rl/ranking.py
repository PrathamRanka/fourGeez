"""Common read-only ranking boundary for baseline and learned strategies."""

from __future__ import annotations

from typing import Protocol

from .contracts import DecisionContext, Recommendation
from .features import FeatureBatch


class Ranker(Protocol):
    """Ranks an already-validated feature batch without changing policy state."""

    strategy: str
    model_version: str

    # rank returns a suggestion and never mutates authorization or payment state.
    def rank(
        self,
        context: DecisionContext,
        batch: FeatureBatch,
    ) -> Recommendation:
        """Return a versioned recommendation for the supplied candidates."""
        ...
