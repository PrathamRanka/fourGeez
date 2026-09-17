"""
recommendation_research
========================

Contextual-bandit recommendation/ranking research workspace for the
negotiation / merchant-selection decision described in the RL design doc:

  - Model:   LinUCB (primary) with an epsilon-greedy fallback, both O(d^2)
             per-update via Sherman-Morrison, no O(d^3) re-inversion.
  - Scope:   the bandit only ever produces a *ranked suggestion*. It has no
             read or write access to trust_tier, the policy engine, or the
             authorization path — see `service.py` for the enforced boundary.
  - Workflow: `simulator.py` generates synthetic evidence/outcome events ->
             `service.py` buffers them like the real SQS-fed pipeline ->
             periodic `retrain()` -> `evaluation.py` scores the result
             against `baseline.py` -> artifacts are versioned and reloadable.

Module map
----------
contracts   : shared dataclasses (UserPrefs, MerchantStats, Option, TrainingEvent)
features    : context/feature vector construction
rewards     : outcome -> scalar reward mapping
bandit      : LinUCBBandit, EpsilonGreedyBandit
baseline    : non-learning heuristic ranker, used as an evaluation floor
service     : training pipeline + the read-only boundary enforced in code
simulator   : synthetic environment for generating training events
evaluation  : offline metrics comparing bandit vs. baseline
cli         : command-line entry point wiring the above together

Only the names in `__all__` are considered part of the stable public API;
internal helpers within each module may change without notice.
"""

from __future__ import annotations

__version__ = "0.1.0"

# NOTE: these imports are the target public API for the package. Each name
# is implemented in the module list above, built out in that order — until
# a given module exists, importing `recommendation_research` will fail on
# that line, which is expected during the scaffold-to-implementation phase.

from .contracts import (
    UserPrefs,
    MerchantStats,
    Option,
    TrainingEvent,
)
from .features import (
    build_feature_vector,
    FEATURE_DIM,
)
from .rewards import (
    Outcome,
    REWARD_MAP,
    reward_from_outcomes,
)
from .bandit import (
    LinUCBBandit,
    EpsilonGreedyBandit,
)
from .baseline import (
    HeuristicBaseline,
)
from .service import (
    RecommendationService,
    TrainingEventBatch,
)
from .simulator import (
    Simulator,
)
from .evaluation import (
    evaluate_policy,
    EvaluationResult,
)

__all__ = [
    "__version__",
    # contracts
    "UserPrefs",
    "MerchantStats",
    "Option",
    "TrainingEvent",
    # features
    "build_feature_vector",
    "FEATURE_DIM",
    # rewards
    "Outcome",
    "REWARD_MAP",
    "reward_from_outcomes",
    # bandit
    "LinUCBBandit",
    "EpsilonGreedyBandit",
    # baseline
    "HeuristicBaseline",
    # service
    "RecommendationService",
    "TrainingEventBatch",
    # simulator
    "Simulator",
    # evaluation
    "evaluate_policy",
    "EvaluationResult",
]
