"""Versioned publication wrapper for reproducible evaluation evidence."""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any

from .evaluation import EvaluationReport

EVALUATION_PUBLICATION_VERSION = "agentpay.evaluation-publication.v1"
SYNTHETIC_DISCLOSURE = "SYNTHETIC DATA - NOT CUSTOMER PERFORMANCE"


@dataclass(frozen=True)
class EvaluationPublication:
    """Binds an evaluation to exact data, model, and contract provenance."""

    schema_version: str
    synthetic: bool
    dataset_version: str
    dataset_seed: int
    dataset_fingerprint: str
    feature_version: str
    reward_config_version: str
    baseline_strategy: str
    baseline_model_version: str
    candidate_strategy: str
    candidate_model_version: str
    model_artifact_version: str
    model_fingerprint: str
    evaluation: EvaluationReport

    # to_dict serializes complete provenance beside the measured results.
    def to_dict(self) -> dict[str, Any]:
        return {
            "schemaVersion": self.schema_version,
            "synthetic": self.synthetic,
            "disclosure": SYNTHETIC_DISCLOSURE,
            "datasetVersion": self.dataset_version,
            "datasetSeed": self.dataset_seed,
            "datasetFingerprint": self.dataset_fingerprint,
            "featureVersion": self.feature_version,
            "rewardConfigVersion": self.reward_config_version,
            "baselineStrategy": self.baseline_strategy,
            "baselineModelVersion": self.baseline_model_version,
            "candidateStrategy": self.candidate_strategy,
            "candidateModelVersion": self.candidate_model_version,
            "modelArtifactVersion": self.model_artifact_version,
            "modelFingerprint": self.model_fingerprint,
            "evaluation": self.evaluation.to_dict(),
        }

    # to_json emits canonical reproducible publication JSON.
    def to_json(self) -> str:
        return json.dumps(self.to_dict(), sort_keys=True, separators=(",", ":"))

    # to_markdown emits provenance before metrics to prevent misleading claims.
    def to_markdown(self) -> str:
        metadata_lines = [
            "# AgentPay R1 evaluation",
            "",
            f"**{SYNTHETIC_DISCLOSURE}**",
            "",
            f"- Dataset version: `{self.dataset_version}`",
            f"- Dataset seed: `{self.dataset_seed}`",
            f"- Dataset fingerprint: `{self.dataset_fingerprint}`",
            f"- Feature version: `{self.feature_version}`",
            f"- Reward configuration: `{self.reward_config_version}`",
            f"- Baseline: `{self.baseline_strategy}` / `{self.baseline_model_version}`",
            f"- Candidate: `{self.candidate_strategy}` / `{self.candidate_model_version}`",
            f"- Model artifact: `{self.model_artifact_version}`",
            f"- Model fingerprint: `{self.model_fingerprint}`",
            "",
        ]
        evaluation_lines = self.evaluation.to_markdown().splitlines()
        return "\n".join(metadata_lines + evaluation_lines) + "\n"


# build_evaluation_publication creates the immutable report provenance envelope.
def build_evaluation_publication(
    *,
    dataset_version: str,
    dataset_seed: int,
    dataset_fingerprint: str,
    feature_version: str,
    reward_config_version: str,
    baseline_strategy: str,
    baseline_model_version: str,
    candidate_strategy: str,
    candidate_model_version: str,
    model_artifact_version: str,
    model_fingerprint: str,
    evaluation: EvaluationReport,
) -> EvaluationPublication:
    if not evaluation.synthetic:
        raise ValueError("R1 publication requires explicitly synthetic evaluation data")
    return EvaluationPublication(
        schema_version=EVALUATION_PUBLICATION_VERSION,
        synthetic=True,
        dataset_version=dataset_version,
        dataset_seed=dataset_seed,
        dataset_fingerprint=dataset_fingerprint,
        feature_version=feature_version,
        reward_config_version=reward_config_version,
        baseline_strategy=baseline_strategy,
        baseline_model_version=baseline_model_version,
        candidate_strategy=candidate_strategy,
        candidate_model_version=candidate_model_version,
        model_artifact_version=model_artifact_version,
        model_fingerprint=model_fingerprint,
        evaluation=evaluation,
    )
