"""Generate reproducible synthetic candidates, contexts, and outcome events.

TODO(RL-004):
- Use a caller-supplied fixed seed.
- Model price-sensitive, reliability-sensitive, and latency-sensitive buyer segments.
- Generate failure, override, and dispute outcomes without embedding them in input features.
- Write generated datasets under rl/data/generated, which remains untracked.
- Label every output as synthetic so it cannot be presented as customer evidence.
"""

