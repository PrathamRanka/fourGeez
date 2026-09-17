"""Validate raw contracts and build deterministic model feature vectors.

TODO(RL-002):
- Filter ineligible, unavailable, and over-budget candidates before ranking.
- Normalize price, latency, delivery reliability, and dispute-rate features.
- Define explicit missing-value behavior; never silently substitute unknown values.
- Keep feature ordering stable and versioned.
- Add golden fixtures proving equivalent inputs produce identical vectors.
"""

