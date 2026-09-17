"""Public API for AgentPay's isolated recommendation research workspace."""

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
    "FeatureBatch",
    "OutcomeEvent",
    "OutcomeType",
    "Recommendation",
    "RejectionReason",
    "__version__",
    "build_feature_batch",
]
