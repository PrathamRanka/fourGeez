# Recommendation research contracts

Status: **Locked for RL-001**.

The recommendation workspace accepts only already-authorized, non-secret facts
and returns a ranking suggestion. These contracts cannot authorize purchases,
change buyer limits, select approval policy, execute payments, or resolve
disputes.

## Versions

| Contract | Version |
|---|---|
| Decision context | `agentpay.recommendation-context.v1` |
| Candidate offer | `agentpay.candidate-offer.v1` |
| Recommendation | `agentpay.recommendation.v1` |
| Outcome event | `agentpay.recommendation-outcome.v1` |
| Feature definition | `agentpay.features.v1` |
| Synthetic dataset | `agentpay.synthetic-dataset.v1` |

Every JSON object rejects unknown and missing fields. A version mismatch is an
error rather than a compatibility fallback.

## Money and rates

`maximumAmount` and candidate `amount` are base-10 atomic-unit strings. They
must contain only digits, have no sign, decimal point, exponent, or leading
zero, and be greater than zero. Money is never converted to floating point.

Preference weights and historical rates are integer basis points from `0` to
`10000`. Preference weights must have a positive total. Latency is an integer
number of milliseconds greater than zero.

## DecisionContext

Required fields are `schemaVersion`, `requestId`, `featureVersion`,
`maximumAmount`, `priceWeightBps`, `qualityWeightBps`, `latencyWeightBps`, and
`candidates`. `buyerSegmentId` is optional and must remain an opaque identifier.
Candidate identifiers must be unique and each request contains 1-50 offers.

## CandidateOffer

Required fields are `schemaVersion`, `offerId`, `sellerId`, `amount`, `asset`,
`network`, `capabilities`, `available`, `eligible`,
`historicalDeliveryRateBps`, `historicalDisputeRateBps`, and `p95LatencyMs`.

Eligibility is input from the authoritative Go policy layer. Later feature
construction may reject unavailable, ineligible, or over-budget candidates;
the recommendation workspace never changes eligibility.

## Recommendation

Required fields are `schemaVersion`, `recommendationId`, `requestId`,
`rankedOfferIds`, `scores`, `reasons`, `strategy`, `modelVersion`, and
`featureVersion`. Scores are finite model values, not money. Ranked identifiers
must be unique and exactly match the score keys.

## OutcomeEvent

Required fields are `schemaVersion`, `eventId`, `recommendationId`, `offerId`,
`eventType`, `occurredAt`, `featureVersion`, `strategyVersion`, and
`modelVersion`. Supported event types are `recommendation.accepted`,
`recommendation.overridden`, `transaction.fulfilled`, `transaction.failed`,
`transaction.disputed`, and `dispute.resolved`. Timestamps are RFC 3339 UTC.

## Forbidden data

Contract parsing recursively rejects fields whose normalized names represent
authorization headers, cookies, payment signatures or proofs, approval tokens,
wallet keys, signing secrets, integration credentials, prompts, raw request
bodies, or raw response bodies.

## Feature vector v1

`agentpay.features.v1` contains seven columns in this fixed order:

1. `price_weight`
2. `quality_weight`
3. `latency_weight`
4. `price_score`
5. `delivery_rate`
6. `dispute_free_rate`
7. `latency_score`

Preference weights and historical rates divide basis points by `10000`.
`price_score` is `1 - amount / maximumAmount`, clamped to `[0, 1]` after the
atomic-unit comparison. `latency_score` is `1 - p95LatencyMs / 300000`, also
clamped to `[0, 1]`.

Unavailable, ineligible, and over-budget candidates are rejected before feature
construction with explicit reason codes. Accepted and rejected candidates are
ordered by `offerId`, so input ordering cannot change the feature batch.

## Deterministic baseline v1

`deterministic-baseline` version `baseline-v1` computes three interpretable
components for each accepted candidate:

- price fit: `price_weight * price_score`;
- quality fit: `quality_weight * mean(delivery_rate, dispute_free_rate)`; and
- latency fit: `latency_weight * latency_score`.

The total score is the sum of those components. Results sort by descending
score and then ascending `offerId`, making ties and candidate-order changes
deterministic. The recommendation identifier is derived from the request ID,
feature version, ranked IDs, and scores. The baseline returns no result when no
candidate survives validation.

## Synthetic dataset v1

Synthetic datasets are always marked with `synthetic: true` and record the
exact random seed. Each scenario contains a pre-decision context, the baseline
recommendation, a segment label, and outcome events generated only after the
recommendation. Supported segments are `cost-sensitive`, `quality-sensitive`,
and `latency-sensitive`. Generation uses local seeded randomness and is bounded
to 1-10,000 scenarios.

## Reward configuration v1

`reward-v1` assigns explicit configurable components to acceptance, override,
fulfillment, failure, dispute, and resolution events. Reward calculation emits
every component with its source event and rejects mixed identities, incomplete
lifecycles, invalid ordering, incompatible versions, and events after an
evaluation cutoff. Rewards are offline measurements only and cannot change
authorization, payment, fulfillment, or dispute decisions.

## Offline LinUCB v1

`offline-linucb` version `linucb-v1` trains only from validated, version-matched
offline examples. Its JSON artifact records the feature version, algorithm,
hyperparameters, seed, training count, and a checksum of the complete learned
state. Live `rank` calls are read-only and the Go backend must still revalidate
every suggested offer before use.

## Evaluation report v1

`agentpay.evaluation-report.v1` compares paired observed outcomes for the
deterministic baseline and a candidate ranker. It reports acceptance, atomic
cost, fulfillment, dispute rate, and aggregate reward with paired 95 percent
confidence intervals and segment breakdowns. Mixed provenance, outcome leakage,
and unobserved candidate outcomes are rejected. Explicit reward and dispute
thresholds determine the report pass/fail result.
