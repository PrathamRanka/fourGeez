"""Define versioned recommendation inputs, outputs, and outcome events.

TODO(RL-001):
- Define DecisionContext, CandidateOffer, Recommendation, and OutcomeEvent.
- Represent money as atomic-unit strings or validated integers, never floats.
- Include feature, strategy, model, and event schema versions.
- Reject unknown fields and invalid ranges.
- Exclude secrets, approval tokens, payment proofs, and unrestricted personal data.
"""

