"""Implement the deterministic offer-ranking baseline.

TODO(RL-003):
- Define a common Ranker protocol used by both baseline and bandit strategies.
- Score only candidates accepted by the feature-validation layer.
- Return ranked offer IDs, component scores, plain-language reasons, and version metadata.
- Make tie-breaking deterministic and independent of input ordering.
- Add golden tests before implementing the contextual bandit.
"""

