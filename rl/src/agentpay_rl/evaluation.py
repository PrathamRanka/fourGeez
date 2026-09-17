"""Paired offline evaluation against the deterministic baseline."""

from __future__ import annotations

import json
import math
from collections.abc import Callable
from dataclasses import dataclass
from statistics import fmean, stdev
from typing import Any

EVALUATION_SCHEMA_VERSION = "agentpay.evaluation-report.v1"
MINIMUM_CONFIDENCE_SAMPLE_COUNT = 30
CONFIDENCE_Z_SCORE = 1.96


class OffPolicyEvaluationError(ValueError):
    """Reports an unsupported comparison without observed paired outcomes."""


@dataclass(frozen=True)
class StrategyOutcome:
    """Contains observable business metrics for one strategy decision."""

    accepted: bool
    amount_atomic: int
    fulfilled: bool
    disputed: bool
    reward: float

    # __post_init__ rejects malformed values before metric calculation.
    def __post_init__(self) -> None:
        if isinstance(self.amount_atomic, bool) or self.amount_atomic < 0:
            raise ValueError("amount_atomic must be a non-negative integer")
        if not math.isfinite(self.reward):
            raise ValueError("reward must be finite")
        if self.disputed and not self.fulfilled:
            raise ValueError("a disputed outcome must first be fulfilled")


@dataclass(frozen=True)
class PairedEvaluationSample:
    """Pairs outcomes for both strategies on the same evaluable scenario."""

    sample_id: str
    segment: str
    synthetic: bool
    baseline: StrategyOutcome
    candidate: StrategyOutcome
    candidate_outcome_observed: bool
    features_contain_outcome: bool = False


@dataclass(frozen=True)
class EvaluationThresholds:
    """Defines explicit release gates for reward and dispute regressions."""

    minimum_reward_delta: float
    maximum_dispute_rate_delta: float

    # default returns conservative no-regression thresholds.
    @classmethod
    def default(cls) -> EvaluationThresholds:
        return cls(
            minimum_reward_delta=0.0,
            maximum_dispute_rate_delta=0.0,
        )


@dataclass(frozen=True)
class MetricComparison:
    """Contains baseline, candidate, paired delta, and a 95% interval."""

    baseline: float
    candidate: float
    delta: float
    confidence_low: float
    confidence_high: float

    # to_dict emits stable machine-readable field names.
    def to_dict(self) -> dict[str, float]:
        return {
            "baseline": self.baseline,
            "candidate": self.candidate,
            "delta": self.delta,
            "confidenceLow": self.confidence_low,
            "confidenceHigh": self.confidence_high,
        }


@dataclass(frozen=True)
class EvaluationReport:
    """Contains global and segment metrics with provenance and release gates."""

    schema_version: str
    synthetic: bool
    sample_count: int
    metrics: dict[str, MetricComparison]
    segment_metrics: dict[str, dict[str, MetricComparison]]
    thresholds: EvaluationThresholds
    passes_thresholds: bool
    regressions: tuple[str, ...]
    warnings: tuple[str, ...]

    # to_dict serializes all provenance, metrics, intervals, and thresholds.
    def to_dict(self) -> dict[str, Any]:
        return {
            "schemaVersion": self.schema_version,
            "synthetic": self.synthetic,
            "sampleCount": self.sample_count,
            "metrics": {
                metric_name: comparison.to_dict()
                for metric_name, comparison in self.metrics.items()
            },
            "segmentMetrics": {
                segment: {
                    metric_name: comparison.to_dict()
                    for metric_name, comparison in metrics.items()
                }
                for segment, metrics in self.segment_metrics.items()
            },
            "thresholds": {
                "minimumRewardDelta": self.thresholds.minimum_reward_delta,
                "maximumDisputeRateDelta": self.thresholds.maximum_dispute_rate_delta,
            },
            "passesThresholds": self.passes_thresholds,
            "regressions": list(self.regressions),
            "warnings": list(self.warnings),
        }

    # to_json emits canonical JSON for reports and command output.
    def to_json(self) -> str:
        return json.dumps(self.to_dict(), sort_keys=True, separators=(",", ":"))

    # to_markdown emits a concise human review with explicit provenance.
    def to_markdown(self) -> str:
        provenance = "SYNTHETIC DATA" if self.synthetic else "REAL OBSERVED DATA"
        lines = [
            "# AgentPay recommendation evaluation",
            "",
            f"**{provenance}**",
            "",
            f"Samples: {self.sample_count}",
            f"Threshold result: {'PASS' if self.passes_thresholds else 'FAIL'}",
            "",
            "| Metric | Baseline | Candidate | Delta | 95% CI |",
            "|---|---:|---:|---:|---:|",
        ]
        for metric_name, comparison in self.metrics.items():
            lines.append(
                "| "
                f"{metric_name} | {comparison.baseline:.6f} | "
                f"{comparison.candidate:.6f} | {comparison.delta:.6f} | "
                f"[{comparison.confidence_low:.6f}, "
                f"{comparison.confidence_high:.6f}] |"
            )
        if self.warnings:
            lines.extend(("", "## Warnings", ""))
            lines.extend(f"- {warning}" for warning in self.warnings)
        if self.regressions:
            lines.extend(("", "## Regressions", ""))
            lines.extend(f"- {regression}" for regression in self.regressions)
        return "\n".join(lines) + "\n"


_METRIC_EXTRACTORS: dict[str, Callable[[StrategyOutcome], float]] = {
    "acceptance_rate": lambda outcome: float(outcome.accepted),
    "average_cost_atomic": lambda outcome: float(outcome.amount_atomic),
    "fulfillment_rate": lambda outcome: float(outcome.fulfilled),
    "dispute_rate": lambda outcome: float(outcome.disputed),
    "aggregate_reward": lambda outcome: outcome.reward,
}


# evaluate_paired_samples computes supported paired metrics and release gates.
def evaluate_paired_samples(
    samples: tuple[PairedEvaluationSample, ...],
    thresholds: EvaluationThresholds,
) -> EvaluationReport:
    _validate_samples(samples)
    metrics = _calculate_metrics(samples)
    segment_metrics = {
        segment: _calculate_metrics(
            tuple(sample for sample in samples if sample.segment == segment)
        )
        for segment in sorted({sample.segment for sample in samples})
    }
    regressions = _find_regressions(metrics, thresholds)
    warnings = _evaluation_warnings(samples)
    return EvaluationReport(
        schema_version=EVALUATION_SCHEMA_VERSION,
        synthetic=samples[0].synthetic,
        sample_count=len(samples),
        metrics=metrics,
        segment_metrics=segment_metrics,
        thresholds=thresholds,
        passes_thresholds=not regressions,
        regressions=regressions,
        warnings=warnings,
    )


# _validate_samples blocks leakage, mixed provenance, and off-policy inference.
def _validate_samples(samples: tuple[PairedEvaluationSample, ...]) -> None:
    if not samples:
        raise ValueError("evaluation samples must not be empty")
    provenance = {sample.synthetic for sample in samples}
    if len(provenance) != 1:
        raise ValueError("evaluation samples must share one provenance")
    sample_ids: set[str] = set()
    for evaluation_sample in samples:
        if not evaluation_sample.sample_id:
            raise ValueError("evaluation sample ID must not be empty")
        if evaluation_sample.sample_id in sample_ids:
            raise ValueError("evaluation sample IDs must be unique")
        sample_ids.add(evaluation_sample.sample_id)
        if not evaluation_sample.segment:
            raise ValueError("evaluation segment must not be empty")
        if evaluation_sample.features_contain_outcome:
            raise ValueError("outcome leakage detected in evaluation features")
        if not evaluation_sample.candidate_outcome_observed:
            raise OffPolicyEvaluationError(
                "candidate outcome is unobserved; off-policy comparison is unsupported"
            )


# _calculate_metrics computes paired means and normal confidence intervals.
def _calculate_metrics(
    samples: tuple[PairedEvaluationSample, ...],
) -> dict[str, MetricComparison]:
    return {
        metric_name: _compare_metric(samples, extractor)
        for metric_name, extractor in _METRIC_EXTRACTORS.items()
    }


# _compare_metric calculates one paired metric and its 95% confidence interval.
def _compare_metric(
    samples: tuple[PairedEvaluationSample, ...],
    extractor: Callable[[StrategyOutcome], float],
) -> MetricComparison:
    baseline_values = [extractor(sample.baseline) for sample in samples]
    candidate_values = [extractor(sample.candidate) for sample in samples]
    paired_deltas = [
        candidate_value - baseline_value
        for baseline_value, candidate_value in zip(
            baseline_values,
            candidate_values,
            strict=True,
        )
    ]
    delta = fmean(paired_deltas)
    margin = 0.0
    if len(paired_deltas) > 1:
        standard_error = stdev(paired_deltas) / math.sqrt(len(paired_deltas))
        margin = CONFIDENCE_Z_SCORE * standard_error
    return MetricComparison(
        baseline=fmean(baseline_values),
        candidate=fmean(candidate_values),
        delta=delta,
        confidence_low=delta - margin,
        confidence_high=delta + margin,
    )


# _find_regressions compares measured deltas with explicit release thresholds.
def _find_regressions(
    metrics: dict[str, MetricComparison],
    thresholds: EvaluationThresholds,
) -> tuple[str, ...]:
    regressions: list[str] = []
    if metrics["aggregate_reward"].delta < thresholds.minimum_reward_delta:
        regressions.append("aggregate_reward")
    if metrics["dispute_rate"].delta > thresholds.maximum_dispute_rate_delta:
        regressions.append("dispute_rate")
    return tuple(regressions)


# _evaluation_warnings reports statistically weak sample and segment counts.
def _evaluation_warnings(
    samples: tuple[PairedEvaluationSample, ...],
) -> tuple[str, ...]:
    warnings: list[str] = []
    if len(samples) < MINIMUM_CONFIDENCE_SAMPLE_COUNT:
        warnings.append(
            f"sample count is below {MINIMUM_CONFIDENCE_SAMPLE_COUNT}; intervals are unstable"
        )
    segments = sorted({sample.segment for sample in samples})
    for segment in segments:
        segment_count = sum(sample.segment == segment for sample in samples)
        if segment_count < MINIMUM_CONFIDENCE_SAMPLE_COUNT:
            warnings.append(
                f"segment {segment} has only {segment_count} samples"
            )
    return tuple(warnings)
