# AgentPay recommendation research workspace

This workspace belongs to the recommendation/RL teammate. It ranks valid seller offers for a buyer agent. It does **not** authorize purchases, change spending limits, execute payments, assign trust tiers, or resolve disputes.

The first deliverable is a deterministic baseline and an offline evaluation harness. A contextual bandit is added only after the baseline and event contracts are tested. Nothing in this directory is on the payment critical path.

## Inputs

The recommender receives only normalized, non-secret facts:

- Buyer preference weights such as cost, latency, and quality priority.
- Maximum amount already approved by the buyer-side policy layer.
- Candidate offer identifiers, prices, capabilities, historical delivery rate, latency, and dispute rate.
- An opaque buyer segment identifier when explicitly allowed; no wallet keys, approval tokens, prompts, or raw personal data.

## Output

The recommender returns:

```json
{
  "recommendationId": "rec_example",
  "rankedOfferIds": ["offer_a", "offer_b"],
  "scores": {"offer_a": 0.82, "offer_b": 0.61},
  "reasons": ["within budget", "higher delivery reliability"],
  "strategy": "deterministic-baseline",
  "modelVersion": "baseline-v1"
}
```

The exact versioned JSON fields and validation rules are defined in
[`CONTRACTS.md`](CONTRACTS.md).

The Go backend independently verifies that every returned offer exists, remains available, is within the approved maximum, and satisfies policy.

## Outcome events

Offline training and evaluation may consume:

- `recommendation.accepted`
- `recommendation.overridden`
- `transaction.fulfilled`
- `transaction.failed`
- `transaction.disputed`
- `dispute.resolved`

Events must contain versioned features and opaque IDs, not secrets or unrestricted request/response bodies.

## Directory map

```text
rl/
  pyproject.toml                    Python tooling and package metadata
  src/agentpay_rl/
    contracts.py                   Versioned input/output/event structures
    features.py                    Validation and feature-vector construction
    baseline.py                    Deterministic ranking strategy
    rewards.py                     Configurable outcome-to-reward mapping
    bandit.py                      Offline contextual-bandit implementation
    simulator.py                   Reproducible synthetic event generation
    evaluation.py                  Baseline-versus-bandit evaluation
    service.py                     JSON adapter used by the Go integration
    cli.py                         Commands for simulation, training, and reports
  tests/                            Unit and deterministic regression tests
  data/                             Local generated data only; real data is never committed
  notebooks/                        Optional exploration; production logic stays in src
```

## Teammate task order

1. Define dataclasses or Pydantic models in `contracts.py`; get schema review before continuing.
2. Add canonical fixtures and strict validation in `features.py`.
3. Implement the deterministic baseline and golden ranking tests.
4. Implement the synthetic simulator with a fixed random seed.
5. Define reward components and report each component separately.
6. Implement an offline contextual bandit behind the same ranking interface.
7. Implement evaluation with confidence intervals and baseline comparison.
8. Add the JSON service adapter only after offline tests pass.
9. Produce a reproducible report; do not claim real-user improvement from synthetic data.

## Definition of done

- Same seed and inputs produce identical results.
- Unknown or missing features fail validation rather than receiving silent defaults.
- Candidate ordering does not affect deterministic scoring.
- Ineligible or over-budget offers are rejected before ranking.
- Evaluation compares against the deterministic baseline, not a random-only baseline.
- Reward components, strategy version, feature version, and dataset version appear in every report.
- No code in this workspace can access payment credentials or call payment APIs.

## Reproducible commands

```powershell
python -m venv .venv
.\.venv\Scripts\Activate.ps1
python -m pip install -e ".[dev]"
pytest
ruff check .
mypy src
agentpay-rl simulate --output data/generated/dataset.json --dataset-version agentpay.synthetic-dataset.v1 --seed 41 --scenario-count 300
agentpay-rl train --input data/generated/dataset.json --output data/generated/model.json --feature-version agentpay.features.v1 --reward-version reward-v1 --seed 41
agentpay-rl evaluate --dataset data/generated/dataset.json --model data/generated/model.json --report-json data/generated/report.json --report-markdown data/generated/report.md --evaluation-version agentpay.evaluation-report.v1 --minimum-reward-delta 0 --maximum-dispute-rate-delta 0
agentpay-rl serve --model data/generated/model.json --host 127.0.0.1 --port 8080
```

Every command requires explicit paths and contract versions and emits dataset,
model, or report fingerprints. `evaluate` exits non-zero when configured
regression thresholds fail. The local HTTP service exposes `POST /recommend`,
`GET /health`, and `GET /metadata`; it does not expose training data.
