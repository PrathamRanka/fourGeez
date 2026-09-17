"""Dependency-free offline LinUCB ranking behind the common Ranker boundary."""

from __future__ import annotations

import hashlib
import json
import math
from dataclasses import asdict, dataclass
from typing import Any

from .contracts import (
    FEATURE_VERSION_V1,
    RECOMMENDATION_SCHEMA_VERSION,
    DecisionContext,
    Recommendation,
)
from .features import FEATURE_DIM, FEATURE_NAMES, FeatureBatch

BANDIT_MODEL_VERSION = "linucb-v1"
BANDIT_STRATEGY_NAME = "offline-linucb"
MODEL_ARTIFACT_VERSION = "agentpay.bandit-artifact.v1"


class BanditError(ValueError):
    """Reports invalid training, ranking, or artifact input."""


class InvalidTrainingExample(BanditError):
    """Reports an offline example that cannot safely train the model."""


class IncompatibleModelVersion(BanditError):
    """Reports an artifact that does not match the supported model contract."""


@dataclass(frozen=True)
class OfflineTrainingExample:
    """Contains one validated feature vector and its offline reward."""

    event_id: str
    feature_version: str
    features: tuple[float, ...]
    reward: float


@dataclass(frozen=True)
class BanditMetadata:
    """Describes the exact deterministic model state and training inputs."""

    algorithm: str
    model_version: str
    feature_version: str
    feature_dimension: int
    seed: int
    ridge: float
    exploration_alpha: float
    training_example_count: int
    state_checksum: str


class LinUCBBandit:
    """Learns linear rewards offline and performs read-only UCB ranking."""

    strategy = BANDIT_STRATEGY_NAME
    model_version = BANDIT_MODEL_VERSION

    # __init__ creates an untrained regularized model with deterministic metadata.
    def __init__(
        self,
        seed: int,
        ridge: float = 1.0,
        exploration_alpha: float = 0.1,
        feature_version: str = FEATURE_VERSION_V1,
    ) -> None:
        if ridge <= 0 or not math.isfinite(ridge):
            raise BanditError("ridge must be finite and greater than zero")
        if exploration_alpha < 0 or not math.isfinite(exploration_alpha):
            raise BanditError("exploration_alpha must be finite and non-negative")
        if feature_version != FEATURE_VERSION_V1:
            raise IncompatibleModelVersion("unsupported feature version")

        self.seed = seed
        self.ridge = ridge
        self.exploration_alpha = exploration_alpha
        self.feature_version = feature_version
        self._design_matrix = _identity_matrix(FEATURE_DIM, ridge)
        self._reward_vector = [0.0 for _ in range(FEATURE_DIM)]
        self._weights = [0.0 for _ in range(FEATURE_DIM)]
        self._training_example_count = 0

    # fit_offline validates the full batch before changing model state.
    def fit_offline(
        self,
        examples: tuple[OfflineTrainingExample, ...],
    ) -> BanditMetadata:
        _validate_training_examples(examples, self.feature_version)

        next_design_matrix = [row.copy() for row in self._design_matrix]
        next_reward_vector = self._reward_vector.copy()
        for example in examples:
            for row_index in range(FEATURE_DIM):
                next_reward_vector[row_index] += (
                    example.reward * example.features[row_index]
                )
                for column_index in range(FEATURE_DIM):
                    next_design_matrix[row_index][column_index] += (
                        example.features[row_index]
                        * example.features[column_index]
                    )

        inverse = _invert_matrix(next_design_matrix)
        next_weights = _matrix_vector_product(inverse, next_reward_vector)
        self._design_matrix = next_design_matrix
        self._reward_vector = next_reward_vector
        self._weights = next_weights
        self._training_example_count += len(examples)
        return self.metadata()

    # rank scores one validated feature batch without updating learned state.
    def rank(
        self,
        context: DecisionContext,
        batch: FeatureBatch,
    ) -> Recommendation:
        _validate_ranking_batch(context, batch)
        if not batch.candidate_ids:
            raise BanditError("no candidate survived feature validation")

        inverse = _invert_matrix(self._design_matrix)
        score_by_offer: dict[str, float] = {}
        for offer_id, feature_row in zip(batch.candidate_ids, batch.rows, strict=True):
            expected_reward = _dot(feature_row, self._weights)
            uncertainty_vector = _matrix_vector_product(inverse, list(feature_row))
            uncertainty = math.sqrt(max(0.0, _dot(feature_row, uncertainty_vector)))
            score_by_offer[offer_id] = round(
                expected_reward + self.exploration_alpha * uncertainty,
                12,
            )

        ranked_offer_ids = tuple(
            sorted(
                score_by_offer,
                key=lambda offer_id: (-score_by_offer[offer_id], offer_id),
            )
        )
        return Recommendation(
            schema_version=RECOMMENDATION_SCHEMA_VERSION,
            recommendation_id=_recommendation_id(
                context,
                ranked_offer_ids,
                score_by_offer,
                self.metadata().state_checksum,
            ),
            request_id=context.request_id,
            ranked_offer_ids=ranked_offer_ids,
            scores={offer_id: score_by_offer[offer_id] for offer_id in ranked_offer_ids},
            reasons=(
                f"{ranked_offer_ids[0]} has the highest offline LinUCB score",
            ),
            strategy=self.strategy,
            model_version=self.model_version,
            feature_version=self.feature_version,
        )

    # metadata returns immutable version and model-state identification.
    def metadata(self) -> BanditMetadata:
        return BanditMetadata(
            algorithm="LinUCB",
            model_version=self.model_version,
            feature_version=self.feature_version,
            feature_dimension=FEATURE_DIM,
            seed=self.seed,
            ridge=self.ridge,
            exploration_alpha=self.exploration_alpha,
            training_example_count=self._training_example_count,
            state_checksum=_state_checksum(
                self._design_matrix,
                self._reward_vector,
                self._weights,
            ),
        )

    # to_json serializes a portable, inspectable artifact without pickle.
    def to_json(self) -> str:
        artifact = {
            "schemaVersion": MODEL_ARTIFACT_VERSION,
            "metadata": _metadata_to_dict(self.metadata()),
            "designMatrix": self._design_matrix,
            "rewardVector": self._reward_vector,
            "weights": self._weights,
        }
        return json.dumps(artifact, sort_keys=True, separators=(",", ":"))

    # from_json validates versions, dimensions, and checksum before loading state.
    @classmethod
    def from_json(cls, serialized_artifact: str) -> LinUCBBandit:
        try:
            artifact = json.loads(serialized_artifact)
        except json.JSONDecodeError:
            raise IncompatibleModelVersion("model artifact is not valid JSON") from None
        if not isinstance(artifact, dict):
            raise IncompatibleModelVersion("model artifact must be a JSON object")
        if artifact.get("schemaVersion") != MODEL_ARTIFACT_VERSION:
            raise IncompatibleModelVersion("unsupported model artifact version")

        metadata = _metadata_from_dict(artifact.get("metadata"))
        design_matrix = _float_matrix(artifact.get("designMatrix"))
        reward_vector = _float_vector(artifact.get("rewardVector"))
        weights = _float_vector(artifact.get("weights"))
        _validate_artifact_state(metadata, design_matrix, reward_vector, weights)

        model = cls(
            seed=metadata.seed,
            ridge=metadata.ridge,
            exploration_alpha=metadata.exploration_alpha,
            feature_version=metadata.feature_version,
        )
        model._design_matrix = design_matrix
        model._reward_vector = reward_vector
        model._weights = weights
        model._training_example_count = metadata.training_example_count
        return model


# _validate_training_examples checks every example before any model mutation.
def _validate_training_examples(
    examples: tuple[OfflineTrainingExample, ...],
    feature_version: str,
) -> None:
    if not examples:
        raise InvalidTrainingExample("training examples must not be empty")
    event_ids: set[str] = set()
    for example in examples:
        if not example.event_id or example.event_id in event_ids:
            raise InvalidTrainingExample("training event IDs must be non-empty and unique")
        event_ids.add(example.event_id)
        if example.feature_version != feature_version:
            raise InvalidTrainingExample("training feature version is incompatible")
        if len(example.features) != FEATURE_DIM:
            raise InvalidTrainingExample("training feature dimension is incompatible")
        if not all(math.isfinite(value) for value in example.features):
            raise InvalidTrainingExample("training features must be finite")
        if not math.isfinite(example.reward):
            raise InvalidTrainingExample("training reward must be finite")


# _validate_ranking_batch checks the context and fixed feature contract.
def _validate_ranking_batch(context: DecisionContext, batch: FeatureBatch) -> None:
    if batch.request_id != context.request_id:
        raise BanditError("feature batch request does not match context")
    if batch.feature_version != context.feature_version:
        raise IncompatibleModelVersion("ranking feature version is incompatible")
    if batch.feature_version != FEATURE_VERSION_V1:
        raise IncompatibleModelVersion("ranking feature version is unsupported")
    if batch.feature_names != FEATURE_NAMES:
        raise BanditError("ranking feature columns are incompatible")
    if len(batch.candidate_ids) != len(batch.rows):
        raise BanditError("ranking candidate and row counts differ")
    for feature_row in batch.rows:
        if len(feature_row) != FEATURE_DIM:
            raise BanditError("ranking feature dimension is incompatible")
        if not all(math.isfinite(value) for value in feature_row):
            raise BanditError("ranking features must be finite")


# _identity_matrix creates the regularized design-matrix starting point.
def _identity_matrix(dimension: int, diagonal: float) -> list[list[float]]:
    return [
        [diagonal if row_index == column_index else 0.0 for column_index in range(dimension)]
        for row_index in range(dimension)
    ]


# _invert_matrix computes a small dense inverse with pivoted Gauss-Jordan elimination.
def _invert_matrix(matrix: list[list[float]]) -> list[list[float]]:
    dimension = len(matrix)
    augmented = [
        row.copy() + identity_row
        for row, identity_row in zip(
            matrix,
            _identity_matrix(dimension, 1.0),
            strict=True,
        )
    ]
    for pivot_index in range(dimension):
        pivot_row_index = max(
            range(pivot_index, dimension),
            key=lambda row_index: abs(augmented[row_index][pivot_index]),
        )
        if abs(augmented[pivot_row_index][pivot_index]) < 1e-12:
            raise BanditError("model design matrix is singular")
        augmented[pivot_index], augmented[pivot_row_index] = (
            augmented[pivot_row_index],
            augmented[pivot_index],
        )
        pivot_value = augmented[pivot_index][pivot_index]
        augmented[pivot_index] = [
            value / pivot_value for value in augmented[pivot_index]
        ]
        for row_index in range(dimension):
            if row_index == pivot_index:
                continue
            scale = augmented[row_index][pivot_index]
            augmented[row_index] = [
                value - scale * pivot_value
                for value, pivot_value in zip(
                    augmented[row_index],
                    augmented[pivot_index],
                    strict=True,
                )
            ]
    return [row[dimension:] for row in augmented]


# _matrix_vector_product multiplies one dense matrix by one vector.
def _matrix_vector_product(matrix: list[list[float]], vector: list[float]) -> list[float]:
    return [_dot(row, vector) for row in matrix]


# _dot calculates a stable scalar product for equal-width vectors.
def _dot(left: tuple[float, ...] | list[float], right: list[float]) -> float:
    return math.fsum(
        left_value * right_value
        for left_value, right_value in zip(left, right, strict=True)
    )


# _state_checksum fingerprints the complete learned numeric state.
def _state_checksum(
    design_matrix: list[list[float]],
    reward_vector: list[float],
    weights: list[float],
) -> str:
    serialized_state = json.dumps(
        {
            "designMatrix": design_matrix,
            "rewardVector": reward_vector,
            "weights": weights,
        },
        sort_keys=True,
        separators=(",", ":"),
    )
    return hashlib.sha256(serialized_state.encode("utf-8")).hexdigest()


# _metadata_to_dict maps Python field names to the versioned artifact shape.
def _metadata_to_dict(metadata: BanditMetadata) -> dict[str, Any]:
    metadata_values = asdict(metadata)
    return {
        "algorithm": metadata_values["algorithm"],
        "modelVersion": metadata_values["model_version"],
        "featureVersion": metadata_values["feature_version"],
        "featureDimension": metadata_values["feature_dimension"],
        "seed": metadata_values["seed"],
        "ridge": metadata_values["ridge"],
        "explorationAlpha": metadata_values["exploration_alpha"],
        "trainingExampleCount": metadata_values["training_example_count"],
        "stateChecksum": metadata_values["state_checksum"],
    }


# _metadata_from_dict validates the exact metadata fields and primitive types.
def _metadata_from_dict(raw_metadata: object) -> BanditMetadata:
    if not isinstance(raw_metadata, dict):
        raise IncompatibleModelVersion("model metadata must be a JSON object")
    required_fields = {
        "algorithm",
        "modelVersion",
        "featureVersion",
        "featureDimension",
        "seed",
        "ridge",
        "explorationAlpha",
        "trainingExampleCount",
        "stateChecksum",
    }
    if set(raw_metadata) != required_fields:
        raise IncompatibleModelVersion("model metadata fields are incompatible")
    try:
        return BanditMetadata(
            algorithm=str(raw_metadata["algorithm"]),
            model_version=str(raw_metadata["modelVersion"]),
            feature_version=str(raw_metadata["featureVersion"]),
            feature_dimension=int(raw_metadata["featureDimension"]),
            seed=int(raw_metadata["seed"]),
            ridge=float(raw_metadata["ridge"]),
            exploration_alpha=float(raw_metadata["explorationAlpha"]),
            training_example_count=int(raw_metadata["trainingExampleCount"]),
            state_checksum=str(raw_metadata["stateChecksum"]),
        )
    except (TypeError, ValueError):
        raise IncompatibleModelVersion("model metadata contains invalid values") from None


# _float_vector converts a JSON array into a finite numeric vector.
def _float_vector(raw_vector: object) -> list[float]:
    if not isinstance(raw_vector, list):
        raise IncompatibleModelVersion("model vector must be a JSON array")
    try:
        vector = [float(value) for value in raw_vector]
    except (TypeError, ValueError):
        raise IncompatibleModelVersion("model vector must contain numbers") from None
    if not all(math.isfinite(value) for value in vector):
        raise IncompatibleModelVersion("model vector must contain finite numbers")
    return vector


# _float_matrix converts a JSON array into a finite square numeric matrix.
def _float_matrix(raw_matrix: object) -> list[list[float]]:
    if not isinstance(raw_matrix, list):
        raise IncompatibleModelVersion("design matrix must be a JSON array")
    return [_float_vector(raw_row) for raw_row in raw_matrix]


# _validate_artifact_state enforces versions, dimensions, and integrity.
def _validate_artifact_state(
    metadata: BanditMetadata,
    design_matrix: list[list[float]],
    reward_vector: list[float],
    weights: list[float],
) -> None:
    if metadata.algorithm != "LinUCB":
        raise IncompatibleModelVersion("unsupported model algorithm")
    if metadata.model_version != BANDIT_MODEL_VERSION:
        raise IncompatibleModelVersion("unsupported model version")
    if metadata.feature_version != FEATURE_VERSION_V1:
        raise IncompatibleModelVersion("unsupported feature version")
    if metadata.feature_dimension != FEATURE_DIM:
        raise IncompatibleModelVersion("model feature dimension is incompatible")
    if len(design_matrix) != FEATURE_DIM:
        raise IncompatibleModelVersion("design matrix dimension is incompatible")
    if any(len(row) != FEATURE_DIM for row in design_matrix):
        raise IncompatibleModelVersion("design matrix row dimension is incompatible")
    if len(reward_vector) != FEATURE_DIM or len(weights) != FEATURE_DIM:
        raise IncompatibleModelVersion("model vector dimension is incompatible")
    checksum = _state_checksum(design_matrix, reward_vector, weights)
    if checksum != metadata.state_checksum:
        raise IncompatibleModelVersion("model state checksum does not match")


# _recommendation_id derives a stable identifier from model state and scores.
def _recommendation_id(
    context: DecisionContext,
    ranked_offer_ids: tuple[str, ...],
    score_by_offer: dict[str, float],
    state_checksum: str,
) -> str:
    canonical = json.dumps(
        {
            "modelVersion": BANDIT_MODEL_VERSION,
            "requestId": context.request_id,
            "rankedOfferIds": ranked_offer_ids,
            "scores": {
                offer_id: format(score_by_offer[offer_id], ".12f")
                for offer_id in ranked_offer_ids
            },
            "stateChecksum": state_checksum,
        },
        sort_keys=True,
        separators=(",", ":"),
    )
    digest = hashlib.sha256(canonical.encode("utf-8")).hexdigest()
    return "rec_" + digest[:26]
