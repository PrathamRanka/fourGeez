"""Translate transaction outcomes into configurable reward components.

TODO(RL-005):
- Represent acceptance, override, fulfillment, failure, dispute, and resolution separately.
- Keep weights in versioned configuration rather than hidden constants.
- Prevent incomplete or future-leaking events from entering offline training.
- Return both aggregate reward and auditable component values.
- Document that reward design cannot change authorization or dispute decisions.
"""

