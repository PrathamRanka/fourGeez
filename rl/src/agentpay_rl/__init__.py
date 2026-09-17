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

__version__ = "0.1.0"

__all__ = [
    "CANDIDATE_OFFER_SCHEMA_VERSION",
    "DECISION_CONTEXT_SCHEMA_VERSION",
    "FEATURE_VERSION_V1",
    "OUTCOME_EVENT_SCHEMA_VERSION",
    "RECOMMENDATION_SCHEMA_VERSION",
    "CandidateOffer",
    "ContractError",
    "DecisionContext",
    "OutcomeEvent",
    "OutcomeType",
    "Recommendation",
    "__version__",
]
