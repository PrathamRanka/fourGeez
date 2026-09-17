# AgentPay R1 evaluation

**SYNTHETIC DATA - NOT CUSTOMER PERFORMANCE**

- Dataset version: `agentpay.synthetic-dataset.v1`
- Dataset seed: `41`
- Dataset fingerprint: `8d89da9b88808d8d6a45e9dcd4e56eea3cf49801890c7836763f6a65b9f975d9`
- Feature version: `agentpay.features.v1`
- Reward configuration: `reward-v1`
- Baseline: `deterministic-baseline` / `baseline-v1`
- Candidate: `offline-linucb` / `linucb-v1`
- Model artifact: `agentpay.bandit-artifact.v1`
- Model fingerprint: `200e223a6f82c78077b36ae6d6bba73a0f9e17797e645db9601cdcd2c4bff789`

# AgentPay recommendation evaluation

**SYNTHETIC DATA**

Samples: 300
Threshold result: PASS

| Metric | Baseline | Candidate | Delta | 95% CI |
|---|---:|---:|---:|---:|
| acceptance_rate | 1.000000 | 0.476667 | -0.523333 | [-0.579946, -0.466720] |
| average_cost_atomic | 541.523333 | 744.923333 | 203.400000 | [172.031213, 234.768787] |
| fulfillment_rate | 0.643333 | 0.906667 | 0.263333 | [0.213409, 0.313257] |
| dispute_rate | 0.193333 | 0.280000 | 0.086667 | [0.041447, 0.131887] |
| aggregate_reward | 0.290000 | 0.668667 | 0.378667 | [0.288993, 0.468340] |
