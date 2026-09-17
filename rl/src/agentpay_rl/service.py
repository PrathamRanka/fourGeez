"""Provide the narrow JSON adapter used by the Go backend.

TODO(RL-008):
- Accept a versioned DecisionContext and CandidateOffer collection.
- Return a versioned Recommendation using the selected Ranker.
- Add request-size, timeout, and candidate-count limits.
- Expose health and model-metadata operations without exposing training data.
- Do not call seller APIs, payment APIs, approval APIs, or AWS Secrets Manager.
"""

