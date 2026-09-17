"""Public API for AgentPay's isolated recommendation research workspace."""

from .baseline import (
    MODEL_VERSION as BASELINE_MODEL_VERSION,
)
from .baseline import (
    STRATEGY_NAME as BASELINE_STRATEGY_NAME,
)
from .baseline import (
    DeterministicBaseline,
    FeatureBatchMismatch,
    NoRankableCandidates,
)
from .contracts import (
    CANDIDATE_OFFER_SCHEMA_VERSION,
    DECISION_CONTEXT_SCHEMA_VERSION,
    FEATURE_VERSION_V1,
    OUTCOME_EVENT_SCHEMA_VERSION,
    RECOMMENDATION_SCHEMA_VERSION,
    CandidateOffer,
    ContractError,
    DecisionContext,
    OutcomeEvent,
    OutcomeType,
    Recommendation,
)
from .features import (
    FEATURE_DIM,
    FEATURE_NAMES,
    CandidateRejection,
    FeatureBatch,
    RejectionReason,
    build_feature_batch,
)

__version__ = "0.1.0"

__all__ = [
    "BASELINE_MODEL_VERSION",
    "BASELINE_STRATEGY_NAME",
    "CANDIDATE_OFFER_SCHEMA_VERSION",
    "DECISION_CONTEXT_SCHEMA_VERSION",
    "FEATURE_DIM",
    "FEATURE_NAMES",
    "FEATURE_VERSION_V1",
    "OUTCOME_EVENT_SCHEMA_VERSION",
    "RECOMMENDATION_SCHEMA_VERSION",
    "CandidateOffer",
    "CandidateRejection",
    "ContractError",
    "DecisionContext",
    "DeterministicBaseline",
    "FeatureBatch",
    "FeatureBatchMismatch",
    "NoRankableCandidates",
    "OutcomeEvent",
    "OutcomeType",
    "Recommendation",
    "RejectionReason",
    "__version__",
    "build_feature_batch",
]
