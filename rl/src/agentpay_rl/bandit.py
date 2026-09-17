"""Implement an offline contextual-bandit ranker behind the common Ranker protocol.

TODO(RL-006):
- Begin with an interpretable algorithm such as LinUCB or epsilon-greedy linear scoring.
- Train only from version-compatible, validated offline events.
- Use deterministic seeds and serialize complete model metadata.
- Refuse to load incompatible feature/model versions.
- Never perform online weight updates in the payment request path.
"""

