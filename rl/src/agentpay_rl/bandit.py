"""Implement an offline contextual-bandit ranker behind the common Ranker protocol.

TODO(RL-006):
- Begin with an interpretable algorithm such as LinUCB or epsilon-greedy linear scoring.
- Train only from version-compatible, validated offline events.
- Use deterministic seeds and serialize complete model metadata.
- Refuse to load incompatible feature/model versions.
- Never perform online weight updates in the payment request path.

Design notes
-------------
This module is intentionally decoupled from `contracts.py` / `features.py`
(not yet built): it operates on raw `(n_candidates, feature_dim)` matrices and
candidate ids rather than on `UserPrefs` / `Option` directly. Whatever builds
those feature vectors (the future `features.py`) is responsible for keeping
`feature_version` in sync with what's stamped into `ModelMetadata` here.

Hard boundary: `rank()` is called from the live decision/payment-request path
and MUST be read-only. All learning happens in `fit_offline()`, which is only
ever invoked by an offline batch job over validated historical events. There
is no code path from `rank()` to `A_inv`/`b`/`theta` mutation.
"""

from __future__ import annotations

import hashlib
import pickle
import time
from dataclasses import dataclass, field
from typing import List, NamedTuple, Protocol, Sequence

import numpy as np

# --------------------------------------------------------------------------- #
# Versioning
# --------------------------------------------------------------------------- #

# Bump MODEL_VERSION on any change to the algorithm/serialization format.
# Bump FEATURE_VERSION (owned by features.py once it exists) on any change
# to what a feature vector's columns mean. A model trained against one
# feature version must never score vectors built under a different one.
MODEL_VERSION = "1.0.0"
DEFAULT_FEATURE_VERSION = "1.0.0"


class IncompatibleModelVersion(Exception):
    """Raised when a loaded artifact's feature/model version doesn't match."""


class InvalidTrainingEvent(Exception):
    """Raised when an offline event fails validation before training."""


# --------------------------------------------------------------------------- #
# Shared contract types (mirrors what contracts.py will eventually own)
# --------------------------------------------------------------------------- #

class RankedCandidate(NamedTuple):
    candidate_id: str
    score: float


class Ranker(Protocol):
    """Minimal read-only interface every ranking model in this package implements."""

    def rank(
        self, feature_matrix: np.ndarray, candidate_ids: Sequence[str]
    ) -> List[RankedCandidate]:
        """Score candidates for one request. MUST NOT mutate model weights."""
        ...


@dataclass(frozen=True)
class OfflineEvent:
    """
    One validated, version-tagged offline training example.

    `features` is the full context+candidate feature vector for the option
    that was actually chosen (i.e. what `features.py` would produce for it),
    and `reward` is the scalar produced by `rewards.reward_from_outcomes`.
    """
    features: np.ndarray
    reward: float
    feature_version: str = DEFAULT_FEATURE_VERSION
    event_id: str = ""


@dataclass(frozen=True)
class ModelMetadata:
    algorithm: str
    model_version: str
    feature_version: str
    feature_dim: int
    seed: int
    hyperparams: dict
    n_training_events: int
    trained_at: float
    weights_checksum: str


# --------------------------------------------------------------------------- #
# Validation
# --------------------------------------------------------------------------- #

def validate_events(
    events: Sequence[OfflineEvent],
    expected_feature_dim: int,
    expected_feature_version: str,
) -> None:
    """
    Reject the whole batch on the first bad event rather than training on
    partially-corrupt data. Called by `fit_offline` before any weight update.
    """
    if not events:
        raise InvalidTrainingEvent("empty event batch")

    for e in events:
        if e.feature_version != expected_feature_version:
            raise InvalidTrainingEvent(
                f"event {e.event_id!r} has feature_version={e.feature_version!r}, "
                f"expected {expected_feature_version!r}"
            )
        if e.features.shape != (expected_feature_dim,):
            raise InvalidTrainingEvent(
                f"event {e.event_id!r} has feature shape {e.features.shape}, "
                f"expected ({expected_feature_dim},)"
            )
        if not np.all(np.isfinite(e.features)):
            raise InvalidTrainingEvent(f"event {e.event_id!r} has non-finite features")
        if not np.isfinite(e.reward):
            raise InvalidTrainingEvent(f"event {e.event_id!r} has non-finite reward")


def _checksum(A_inv: np.ndarray, b: np.ndarray) -> str:
    h = hashlib.sha256()
    h.update(A_inv.tobytes())
    h.update(b.tobytes())
    return h.hexdigest()[:16]


# --------------------------------------------------------------------------- #
# Shared ridge-regression base
# --------------------------------------------------------------------------- #

class _LinearRidgeBase:
    """
    Common state/serialization for linear contextual bandits: a ridge design
    matrix inverse (`A_inv`) and reward-weighted feature sum (`b`), updated
    incrementally via Sherman-Morrison (O(d^2) per event, no O(d^3) inversion).

    Weights (`A_inv`, `b`, and the derived `theta`) are only ever written by
    `fit_offline`. `rank()` implementations in subclasses only read `theta`.
    """

    algorithm_name = "linear_ridge_base"

    def __init__(
        self,
        feature_dim: int,
        ridge: float = 1.0,
        seed: int = 0,
        feature_version: str = DEFAULT_FEATURE_VERSION,
        hyperparams: dict | None = None,
    ):
        self.feature_dim = feature_dim
        self.feature_version = feature_version
        self.seed = seed
        self._hyperparams = dict(hyperparams or {})
        self._hyperparams.setdefault("ridge", ridge)

        self.A_inv = np.eye(feature_dim) / ridge
        self.b = np.zeros(feature_dim)
        self.theta = np.zeros(feature_dim)  # precomputed, read-only at request time

        self._rng = np.random.default_rng(seed)
        self.n_training_events = 0
        self.trained_at = 0.0

    def _assert_request_time_read_only(self) -> None:
        """Tripwire: fails loudly if a future edit lets rank() reach a mutator."""
        import inspect
        for frame_info in inspect.stack():
            if frame_info.function in ("fit_offline", "_apply_update"):
                raise RuntimeError(
                    "Boundary violation: weight-mutating call reached from rank()"
                )

    def fit_offline(self, events: Sequence[OfflineEvent]) -> ModelMetadata:
        """
        The ONLY path that changes model weights. Validates the batch, applies
        sequential Sherman-Morrison updates, recomputes theta once, and returns
        fresh metadata. Never call this from a live ranking/request path.
        """
        validate_events(events, self.feature_dim, self.feature_version)

        for e in events:
            self._apply_update(e.features, float(e.reward))

        self.theta = self.A_inv @ self.b
        self.n_training_events += len(events)
        self.trained_at = time.time()
        return self.metadata()

    def _apply_update(self, x: np.ndarray, reward: float) -> None:
        Ax = self.A_inv @ x
        denom = 1.0 + x @ Ax
        self.A_inv -= np.outer(Ax, Ax) / denom
        self.b += reward * x

    def metadata(self) -> ModelMetadata:
        return ModelMetadata(
            algorithm=self.algorithm_name,
            model_version=MODEL_VERSION,
            feature_version=self.feature_version,
            feature_dim=self.feature_dim,
            seed=self.seed,
            hyperparams=dict(self._hyperparams),
            n_training_events=self.n_training_events,
            trained_at=self.trained_at,
            weights_checksum=_checksum(self.A_inv, self.b),
        )

    def save(self, path: str) -> ModelMetadata:
        meta = self.metadata()
        payload = {
            "metadata": meta,
            "A_inv": self.A_inv,
            "b": self.b,
            "theta": self.theta,
        }
        with open(path, "wb") as f:
            pickle.dump(payload, f)
        return meta

    @classmethod
    def load(
        cls,
        path: str,
        expected_feature_version: str = DEFAULT_FEATURE_VERSION,
        expected_model_version: str = MODEL_VERSION,
    ) -> "_LinearRidgeBase":
        with open(path, "rb") as f:
            payload = pickle.load(f)
        meta: ModelMetadata = payload["metadata"]

        if meta.feature_version != expected_feature_version:
            raise IncompatibleModelVersion(
                f"artifact feature_version={meta.feature_version!r} != "
                f"expected {expected_feature_version!r}"
            )
        if meta.model_version != expected_model_version:
            raise IncompatibleModelVersion(
                f"artifact model_version={meta.model_version!r} != "
                f"expected {expected_model_version!r}"
            )

        actual_checksum = _checksum(payload["A_inv"], payload["b"])
        if actual_checksum != meta.weights_checksum:
            raise IncompatibleModelVersion(
                f"weights checksum mismatch: artifact may be corrupt "
                f"(expected {meta.weights_checksum}, got {actual_checksum})"
            )

        # Hyperparam keys (e.g. "alpha" for LinUCB, "epsilon" for epsilon-greedy)
        # are named to match each subclass's __init__ kwargs exactly, so they
        # can be splatted straight through without subclass-specific branching.
        extra_kwargs = {k: v for k, v in meta.hyperparams.items() if k != "ridge"}
        model = cls(
            feature_dim=meta.feature_dim,
            ridge=meta.hyperparams.get("ridge", 1.0),
            seed=meta.seed,
            feature_version=meta.feature_version,
            **extra_kwargs,
        )
        model.A_inv = payload["A_inv"]
        model.b = payload["b"]
        model.theta = payload["theta"]
        model.n_training_events = meta.n_training_events
        model.trained_at = meta.trained_at
        return model


# --------------------------------------------------------------------------- #
# LinUCB
# --------------------------------------------------------------------------- #

class LinUCBBandit(_LinearRidgeBase):
    """
    theta = A_inv @ b gives the point estimate of reward for a feature vector;
    the UCB bonus `alpha * sqrt(x^T A_inv x)` rewards exploring candidates the
    model is still uncertain about. Fully vectorized across candidates.
    """

    algorithm_name = "linucb"

    def __init__(
        self,
        feature_dim: int,
        alpha: float = 1.0,
        ridge: float = 1.0,
        seed: int = 0,
        feature_version: str = DEFAULT_FEATURE_VERSION,
    ):
        super().__init__(
            feature_dim, ridge=ridge, seed=seed, feature_version=feature_version,
            hyperparams={"alpha": alpha, "ridge": ridge},
        )
        self.alpha = alpha

    def rank(
        self, feature_matrix: np.ndarray, candidate_ids: Sequence[str]
    ) -> List[RankedCandidate]:
        if feature_matrix.shape[0] != len(candidate_ids):
            raise ValueError("feature_matrix rows must match len(candidate_ids)")
        if feature_matrix.shape[1] != self.feature_dim:
            raise ValueError(
                f"feature dim {feature_matrix.shape[1]} != model dim {self.feature_dim}"
            )

        mean = feature_matrix @ self.theta
        var = np.einsum("ij,jk,ik->i", feature_matrix, self.A_inv, feature_matrix)
        var = np.clip(var, 0.0, None)
        scores = mean + self.alpha * np.sqrt(var)

        order = np.argsort(-scores)
        return [RankedCandidate(candidate_ids[i], float(scores[i])) for i in order]


# --------------------------------------------------------------------------- #
# Epsilon-greedy (simpler fallback)
# --------------------------------------------------------------------------- #

class EpsilonGreedyBandit(_LinearRidgeBase):
    """
    Exploits the ridge-regression point estimate (1 - epsilon) of the time;
    with probability epsilon it shuffles the ranking to explore. Exploration
    randomness is drawn from a seeded RNG for reproducibility across a fresh
    `load()` of the same artifact, not across a single long-lived process.
    """

    algorithm_name = "epsilon_greedy_linear"

    def __init__(
        self,
        feature_dim: int,
        epsilon: float = 0.1,
        ridge: float = 1.0,
        seed: int = 0,
        feature_version: str = DEFAULT_FEATURE_VERSION,
    ):
        super().__init__(
            feature_dim, ridge=ridge, seed=seed, feature_version=feature_version,
            hyperparams={"epsilon": epsilon, "ridge": ridge},
        )
        self.epsilon = epsilon

    def rank(
        self, feature_matrix: np.ndarray, candidate_ids: Sequence[str]
    ) -> List[RankedCandidate]:
        if feature_matrix.shape[0] != len(candidate_ids):
            raise ValueError("feature_matrix rows must match len(candidate_ids)")
        if feature_matrix.shape[1] != self.feature_dim:
            raise ValueError(
                f"feature dim {feature_matrix.shape[1]} != model dim {self.feature_dim}"
            )

        scores = feature_matrix @ self.theta
        if self._rng.random() < self.epsilon:
            scores = scores.copy()
            self._rng.shuffle(scores)

        order = np.argsort(-scores)
        return [RankedCandidate(candidate_ids[i], float(scores[i])) for i in order]


# --------------------------------------------------------------------------- #
# Demo / smoke test
# --------------------------------------------------------------------------- #

if __name__ == "__main__":
    rng = np.random.default_rng(42)
    dim = 8

    def synth_events(n: int) -> list[OfflineEvent]:
        out = []
        for i in range(n):
            x = rng.normal(size=dim)
            true_theta = np.array([1.0, -0.5, 0.3, 0.0, 0.2, -0.1, 0.4, 0.0])
            reward = float(x @ true_theta + rng.normal(scale=0.1))
            out.append(OfflineEvent(features=x, reward=reward, event_id=f"evt_{i}"))
        return out

    model = LinUCBBandit(feature_dim=dim, alpha=0.5, seed=7)
    print("Before training, metadata:", model.metadata())

    meta = model.fit_offline(synth_events(200))
    print("\nAfter training, metadata:", meta)

    candidates = rng.normal(size=(4, dim))
    candidate_ids = ["a", "b", "c", "d"]
    print("\nRanking:", model.rank(candidates, candidate_ids))

    path = "/tmp/linucb_artifact.pkl"
    model.save(path)
    reloaded = LinUCBBandit.load(path)
    print("Reloaded ranking (should match):", reloaded.rank(candidates, candidate_ids))

    # Confirm version enforcement actually refuses a mismatch.
    try:
        LinUCBBandit.load(path, expected_feature_version="9.9.9")
        print("ERROR: should have raised IncompatibleModelVersion")
    except IncompatibleModelVersion as exc:
        print(f"\nCorrectly refused incompatible version: {exc}")

    # Confirm bad events are rejected before any weight mutation.
    bad_event = OfflineEvent(features=np.array([np.nan] * dim), reward=1.0)
    before_checksum = model.metadata().weights_checksum
    try:
        model.fit_offline([bad_event])
        print("ERROR: should have raised InvalidTrainingEvent")
    except InvalidTrainingEvent as exc:
        after_checksum = model.metadata().weights_checksum
        assert before_checksum == after_checksum, "weights mutated despite validation failure!"
        print(f"Correctly rejected invalid event without mutating weights: {exc}")