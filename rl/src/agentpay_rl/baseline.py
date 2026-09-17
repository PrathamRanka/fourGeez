"""Implement the deterministic offer-ranking baseline.

TODO(RL-003):
- Define a common Ranker protocol used by both baseline and bandit strategies.
- Score only candidates accepted by the feature-validation layer.
- Return ranked offer IDs, component scores, plain-language reasons, and version metadata.
- Make tie-breaking deterministic and independent of input ordering.
- Add golden tests before implementing the contextual bandit.

Design notes
-------------
This is the canonical (for now) home of the `Ranker` protocol and the shared
`FeatureBatch` / `RankingResult` types, since this ticket (RL-003) precedes
the bandit ticket (RL-006). Once `contracts.py` exists, these should move
there and both `baseline.py` and `bandit.py` should import them from
`contracts.py` instead — `bandit.py` currently has its own local
`Ranker`/`RankedCandidate`, which should be reconciled with this module
rather than left to diverge.

`FeatureBatch` carries both the raw `(n_candidates, feature_dim)` matrix
(what `bandit.py` scores) and its column names (what this baseline needs to
compute interpretable per-component scores and reasons). Until `features.py`
exists, `DEFAULT_FEATURE_NAMES` below documents the column layout this
baseline expects; `features.py` is the eventual source of truth and must
keep its output in that order (or `HeuristicBaseline` must be updated).
"""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import Dict, List, Protocol, Sequence, Tuple

import numpy as np

BASELINE_VERSION = "1.0.0"

# Canonical named columns this baseline needs to be interpretable. features.py
# (not yet built) must emit at least these columns, in whatever order it likes,
# as long as `feature_names` on the FeatureBatch says where each one lives.
PRICE_SENSITIVITY = "price_sensitivity"
QUALITY_BAR = "quality_bar"
DELIVERY_URGENCY = "delivery_urgency"
MERCHANT_RELIABILITY = "merchant_reliability"
MERCHANT_DISPUTE_RATE = "merchant_dispute_rate"
MERCHANT_FULFILLMENT_RATE = "merchant_fulfillment_rate"
OPTION_PRICE_SCORE = "option_price_score"
OPTION_QUALITY_SCORE = "option_quality_score"
OPTION_DELIVERY_SCORE = "option_delivery_score"

REQUIRED_FEATURE_NAMES = (
    PRICE_SENSITIVITY,
    QUALITY_BAR,
    DELIVERY_URGENCY,
    MERCHANT_RELIABILITY,
    MERCHANT_DISPUTE_RATE,
    MERCHANT_FULFILLMENT_RATE,
    OPTION_PRICE_SCORE,
    OPTION_QUALITY_SCORE,
    OPTION_DELIVERY_SCORE,
)

# A minimal, order-matching default — used by this module's own demo/golden
# tests only, since features.py doesn't exist yet to supply the real thing.
DEFAULT_FEATURE_NAMES: Tuple[str, ...] = REQUIRED_FEATURE_NAMES


class MissingFeatureColumn(Exception):
    """Raised when a FeatureBatch doesn't carry a column this ranker needs."""


# --------------------------------------------------------------------------- #
# Shared types (Ranker protocol + I/O contracts)
# --------------------------------------------------------------------------- #

@dataclass(frozen=True)
class FeatureBatch:
    """What `features.py` will eventually produce for one ranking request."""
    feature_matrix: np.ndarray          # (n_candidates, feature_dim)
    candidate_ids: Sequence[str]
    feature_names: Sequence[str]        # len == feature_dim, column labels
    feature_version: str


@dataclass(frozen=True)
class ScoreComponent:
    name: str
    raw_value: float
    weight: float
    contribution: float                 # raw_value * weight, signed


@dataclass(frozen=True)
class RankedOffer:
    offer_id: str
    total_score: float
    components: List[ScoreComponent]
    reasons: List[str]                  # plain-language, most-influential first


@dataclass(frozen=True)
class RankingResult:
    ranked_offers: List[RankedOffer]
    dropped_candidate_ids: List[str]    # rejected by feature validation
    ranker_name: str
    ranker_version: str
    feature_version: str
    generated_at: float


class Ranker(Protocol):
    """Shared read-only interface for baseline and bandit ranking strategies."""

    name: str
    version: str

    def rank(self, batch: FeatureBatch) -> RankingResult:
        """Score and order every valid candidate in `batch`. Must not mutate state."""
        ...


# --------------------------------------------------------------------------- #
# Feature validation layer (stand-in until features.py owns this)
# --------------------------------------------------------------------------- #

def validate_candidates(batch: FeatureBatch) -> Tuple[FeatureBatch, List[str]]:
    """
    Drop any candidate row that's malformed (wrong width, non-finite values)
    rather than letting it silently corrupt a score. Returns a new FeatureBatch
    containing only the accepted rows, plus the list of dropped candidate ids.
    """
    if batch.feature_matrix.shape[0] != len(batch.candidate_ids):
        raise ValueError("feature_matrix row count must match len(candidate_ids)")
    if batch.feature_matrix.shape[1] != len(batch.feature_names):
        raise ValueError("feature_matrix column count must match len(feature_names)")

    keep_mask = np.all(np.isfinite(batch.feature_matrix), axis=1)
    dropped_ids = [cid for cid, keep in zip(batch.candidate_ids, keep_mask) if not keep]

    if all(keep_mask):
        return batch, dropped_ids

    valid_batch = FeatureBatch(
        feature_matrix=batch.feature_matrix[keep_mask],
        candidate_ids=[cid for cid, keep in zip(batch.candidate_ids, keep_mask) if keep],
        feature_names=batch.feature_names,
        feature_version=batch.feature_version,
    )
    return valid_batch, dropped_ids


# --------------------------------------------------------------------------- #
# Deterministic heuristic baseline
# --------------------------------------------------------------------------- #

DEFAULT_WEIGHTS: Dict[str, float] = {
    "price_fit": 1.0,          # price_sensitivity * option_price_score
    "quality_fit": 1.0,        # quality_bar * option_quality_score
    "delivery_fit": 1.0,       # delivery_urgency * option_delivery_score
    "reliability": 0.5,        # merchant_reliability, independent of user prefs
    "dispute_penalty": -1.0,   # merchant_dispute_rate, always a negative pull
}


class HeuristicBaseline:
    """
    A fully deterministic, non-learning ranker: total_score is a fixed linear
    combination of interpretable components. No hidden state, no randomness —
    this exists to give the bandit something concrete to beat in evaluation.py,
    and to have a sane fallback ranking if the bandit is unavailable.
    """

    name = "heuristic_baseline"
    version = BASELINE_VERSION

    def __init__(self, weights: Dict[str, float] | None = None):
        self.weights = dict(DEFAULT_WEIGHTS if weights is None else weights)

    def _column_index(self, batch: FeatureBatch, column: str) -> int:
        try:
            return list(batch.feature_names).index(column)
        except ValueError as exc:
            raise MissingFeatureColumn(
                f"FeatureBatch is missing required column {column!r}"
            ) from exc

    def rank(self, batch: FeatureBatch) -> RankingResult:
        for required in REQUIRED_FEATURE_NAMES:
            self._column_index(batch, required)  # raises MissingFeatureColumn early

        valid_batch, dropped_ids = validate_candidates(batch)

        idx = {name: self._column_index(valid_batch, name) for name in REQUIRED_FEATURE_NAMES}
        M = valid_batch.feature_matrix

        offers: List[RankedOffer] = []
        for row_i, offer_id in enumerate(valid_batch.candidate_ids):
            row = M[row_i]

            price_fit = row[idx[PRICE_SENSITIVITY]] * row[idx[OPTION_PRICE_SCORE]]
            quality_fit = row[idx[QUALITY_BAR]] * row[idx[OPTION_QUALITY_SCORE]]
            delivery_fit = row[idx[DELIVERY_URGENCY]] * row[idx[OPTION_DELIVERY_SCORE]]
            reliability = row[idx[MERCHANT_RELIABILITY]]
            dispute_rate = row[idx[MERCHANT_DISPUTE_RATE]]

            raw_values = {
                "price_fit": price_fit,
                "quality_fit": quality_fit,
                "delivery_fit": delivery_fit,
                "reliability": reliability,
                "dispute_penalty": dispute_rate,
            }

            components = [
                ScoreComponent(
                    name=name,
                    raw_value=float(raw_values[name]),
                    weight=float(self.weights[name]),
                    contribution=float(raw_values[name] * self.weights[name]),
                )
                for name in raw_values
            ]
            total_score = float(sum(c.contribution for c in components))

            reasons = self._build_reasons(components, row, idx)

            offers.append(
                RankedOffer(
                    offer_id=offer_id,
                    total_score=total_score,
                    components=components,
                    reasons=reasons,
                )
            )

        # Deterministic, order-independent tie-break: sort by (-score, offer_id).
        # offer_id is a stable identity, not a position in the input list, so
        # re-ordering the input batch never changes the output ordering.
        offers.sort(key=lambda o: (-o.total_score, o.offer_id))

        return RankingResult(
            ranked_offers=offers,
            dropped_candidate_ids=dropped_ids,
            ranker_name=self.name,
            ranker_version=self.version,
            feature_version=batch.feature_version,
            generated_at=time.time(),
        )

    @staticmethod
    def _build_reasons(
        components: List[ScoreComponent], row: np.ndarray, idx: Dict[str, int]
    ) -> List[str]:
        # Top two components by absolute contribution drive the plain-language
        # explanation; sorted by name as a tie-break so this is deterministic too.
        ranked_components = sorted(
            components, key=lambda c: (-abs(c.contribution), c.name)
        )

        templates = {
            "price_fit": "matches your price sensitivity well" if 
                ranked_components[0].contribution >= 0 else
                "is a weaker price fit given your stated sensitivity",
            "quality_fit": "meets your quality bar" if
                ranked_components[0].contribution >= 0 else
                "falls short of your quality bar",
            "delivery_fit": "fits your delivery urgency" if
                ranked_components[0].contribution >= 0 else
                "is slower than your delivery urgency prefers",
            "reliability": "comes from a highly reliable merchant",
            "dispute_penalty": "has an elevated merchant dispute rate",
        }

        reasons: List[str] = []
        for c in ranked_components[:2]:
            reasons.append(templates[c.name])

        dispute_rate = row[idx[MERCHANT_DISPUTE_RATE]]
        if dispute_rate > 0.1 and "has an elevated merchant dispute rate" not in reasons:
            reasons.append("has an elevated merchant dispute rate")

        return reasons


# --------------------------------------------------------------------------- #
# Golden tests (RL-003: "Add golden tests before implementing the contextual
# bandit"). These are inline for now; once a tests/ scaffold exists this
# should move to tests/test_baseline.py using the same fixture below.
# --------------------------------------------------------------------------- #

def _golden_fixture() -> FeatureBatch:
    """
    Fixed, hand-computable inputs: one cheap/lower-quality/slow option, one
    premium option, one balanced option — for a price-sensitive, low-urgency
    user. Values are chosen so the expected ranking is unambiguous by hand.
    """
    # columns: [price_sens, quality_bar, delivery_urgency,
    #           reliability, dispute_rate, fulfillment_rate,
    #           price_score, quality_score, delivery_score]
    user = [0.9, 0.3, 0.1]
    rows = {
        "cheap":   user + [0.7, 0.05, 0.9] + [0.9, 0.4, 0.3],
        "premium": user + [0.95, 0.01, 0.98] + [0.2, 0.95, 0.9],
        "balanced": user + [0.85, 0.03, 0.92] + [0.6, 0.6, 0.6],
    }
    ids = list(rows.keys())
    matrix = np.array([rows[i] for i in ids])
    return FeatureBatch(
        feature_matrix=matrix,
        candidate_ids=ids,
        feature_names=list(DEFAULT_FEATURE_NAMES),
        feature_version="golden-fixture-1",
    )


def _run_golden_tests() -> None:
    baseline = HeuristicBaseline()

    fixture = _golden_fixture()
    result = baseline.rank(fixture)
    order = [o.offer_id for o in result.ranked_offers]

    # Hand-computable expectation: this user is highly price-sensitive and
    # barely urgency-sensitive, so "cheap" should win comfortably; "premium"
    # scores worst on price_fit and only partly recovers on reliability.
    assert order == ["cheap", "balanced", "premium"], f"unexpected order: {order}"

    scores = {o.offer_id: round(o.total_score, 6) for o in result.ranked_offers}
    expected_scores = {
        "cheap": round(0.9 * 0.9 + 0.3 * 0.4 + 0.1 * 0.3 + 0.5 * 0.7 - 0.05, 6),
        "premium": round(0.9 * 0.2 + 0.3 * 0.95 + 0.1 * 0.9 + 0.5 * 0.95 - 0.01, 6),
        "balanced": round(0.9 * 0.6 + 0.3 * 0.6 + 0.1 * 0.6 + 0.5 * 0.85 - 0.03, 6),
    }
    assert scores == expected_scores, f"score mismatch: {scores} != {expected_scores}"

    # Order-independence: reversing the input candidate order must not change
    # the output ranking (this is what the (-score, offer_id) sort guarantees).
    reversed_fixture = FeatureBatch(
        feature_matrix=fixture.feature_matrix[::-1].copy(),
        candidate_ids=list(reversed(fixture.candidate_ids)),
        feature_names=fixture.feature_names,
        feature_version=fixture.feature_version,
    )
    reversed_result = baseline.rank(reversed_fixture)
    reversed_order = [o.offer_id for o in reversed_result.ranked_offers]
    assert reversed_order == order, "ranking depended on input ordering!"

    # Validation layer: a NaN row must be dropped, not scored or crashing the run.
    bad_matrix = np.vstack([fixture.feature_matrix, [np.nan] * fixture.feature_matrix.shape[1]])
    bad_fixture = FeatureBatch(
        feature_matrix=bad_matrix,
        candidate_ids=list(fixture.candidate_ids) + ["broken"],
        feature_names=fixture.feature_names,
        feature_version=fixture.feature_version,
    )
    bad_result = baseline.rank(bad_fixture)
    assert bad_result.dropped_candidate_ids == ["broken"]
    assert "broken" not in [o.offer_id for o in bad_result.ranked_offers]

    # Missing-column safety: a batch that doesn't carry a required column
    # must raise rather than silently scoring on garbage indices.
    incomplete_fixture = FeatureBatch(
        feature_matrix=fixture.feature_matrix[:, :-1],
        candidate_ids=fixture.candidate_ids,
        feature_names=list(fixture.feature_names[:-1]),
        feature_version=fixture.feature_version,
    )
    try:
        baseline.rank(incomplete_fixture)
        raise AssertionError("expected MissingFeatureColumn")
    except MissingFeatureColumn:
        pass

    print("All golden tests passed.")
    print(f"\nGolden ranking: {order}")
    for o in result.ranked_offers:
        print(f"  {o.offer_id}: score={o.total_score:.4f} reasons={o.reasons}")


if __name__ == "__main__":
    _run_golden_tests()