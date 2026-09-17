"""Reproducible command surface for AgentPay recommendation research."""

from __future__ import annotations

import argparse
import hashlib
import json
from argparse import ArgumentParser, Namespace
from collections.abc import Sequence
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

from .bandit import (
    BANDIT_MODEL_VERSION,
    BANDIT_STRATEGY_NAME,
    MODEL_ARTIFACT_VERSION,
    LinUCBBandit,
    OfflineTrainingExample,
)
from .baseline import MODEL_VERSION as BASELINE_MODEL_VERSION
from .baseline import STRATEGY_NAME as BASELINE_STRATEGY_NAME
from .contracts import FEATURE_VERSION_V1, DecisionContext
from .evaluation import (
    EVALUATION_SCHEMA_VERSION,
    EvaluationThresholds,
    PairedEvaluationSample,
    StrategyOutcome,
    evaluate_paired_samples,
)
from .features import build_feature_batch
from .reporting import build_evaluation_publication
from .rewards import REWARD_CONFIG_VERSION, RewardConfig, calculate_reward
from .service import RecommendationService, ServiceError
from .simulator import DATASET_SCHEMA_VERSION, SyntheticDataset, generate_synthetic_dataset


# build_parser defines the explicit, versioned production command surface.
def build_parser() -> ArgumentParser:
    parser = argparse.ArgumentParser(prog="agentpay-rl")
    subparsers = parser.add_subparsers(dest="command", required=True)

    simulate_parser = subparsers.add_parser("simulate")
    simulate_parser.add_argument("--output", type=Path, required=True)
    simulate_parser.add_argument("--dataset-version", required=True)
    simulate_parser.add_argument("--seed", type=int, required=True)
    simulate_parser.add_argument("--scenario-count", type=int, required=True)

    train_parser = subparsers.add_parser("train")
    train_parser.add_argument("--input", type=Path, required=True)
    train_parser.add_argument("--output", type=Path, required=True)
    train_parser.add_argument("--feature-version", required=True)
    train_parser.add_argument("--reward-version", required=True)
    train_parser.add_argument("--seed", type=int, required=True)
    train_parser.add_argument("--ridge", type=float, default=1.0)
    train_parser.add_argument("--exploration-alpha", type=float, default=0.1)

    evaluate_parser = subparsers.add_parser("evaluate")
    evaluate_parser.add_argument("--dataset", type=Path, required=True)
    evaluate_parser.add_argument("--model", type=Path, required=True)
    evaluate_parser.add_argument("--report-json", type=Path, required=True)
    evaluate_parser.add_argument("--report-markdown", type=Path, required=True)
    evaluate_parser.add_argument("--evaluation-version", required=True)
    evaluate_parser.add_argument("--minimum-reward-delta", type=float, required=True)
    evaluate_parser.add_argument("--maximum-dispute-rate-delta", type=float, required=True)

    serve_parser = subparsers.add_parser("serve")
    serve_parser.add_argument("--model", type=Path, required=True)
    serve_parser.add_argument("--host", required=True)
    serve_parser.add_argument("--port", type=int, required=True)
    return parser


# main dispatches one command and emits one machine-readable result.
def main(arguments: Sequence[str] | None = None) -> int:
    parsed_arguments = build_parser().parse_args(arguments)
    try:
        result = _dispatch(parsed_arguments)
    except (OSError, ServiceError, TypeError, ValueError) as error:
        print(
            json.dumps(
                {"status": "error", "message": str(error)},
                sort_keys=True,
            )
        )
        return 1
    if result is not None:
        print(json.dumps(result, sort_keys=True))
    return 0


# _dispatch routes parsed arguments to the selected command implementation.
def _dispatch(arguments: Namespace) -> dict[str, Any] | None:
    if arguments.command == "simulate":
        return _simulate(arguments)
    if arguments.command == "train":
        return _train(arguments)
    if arguments.command == "evaluate":
        return _evaluate(arguments)
    if arguments.command == "serve":
        _serve(arguments)
        return None
    raise ValueError(f"unsupported command: {arguments.command}")


# _simulate writes a canonical synthetic dataset and reports its fingerprint.
def _simulate(arguments: Namespace) -> dict[str, Any]:
    if arguments.dataset_version != DATASET_SCHEMA_VERSION:
        raise ValueError("unsupported dataset version")
    dataset = generate_synthetic_dataset(
        seed=arguments.seed,
        scenario_count=arguments.scenario_count,
    )
    _write_text(arguments.output, dataset.to_json() + "\n")
    return {
        "status": "ok",
        "command": "simulate",
        "datasetVersion": dataset.schema_version,
        "datasetFingerprint": dataset.fingerprint,
        "scenarioCount": len(dataset.scenarios),
        "output": str(arguments.output),
    }


# _train converts completed synthetic outcomes into validated offline examples.
def _train(arguments: Namespace) -> dict[str, Any]:
    if arguments.feature_version != FEATURE_VERSION_V1:
        raise ValueError("unsupported feature version")
    if arguments.reward_version != REWARD_CONFIG_VERSION:
        raise ValueError("unsupported reward version")
    dataset = _read_dataset(arguments.input)
    examples = _training_examples(dataset, RewardConfig.default())
    model = LinUCBBandit(
        seed=arguments.seed,
        ridge=arguments.ridge,
        exploration_alpha=arguments.exploration_alpha,
        feature_version=arguments.feature_version,
    )
    metadata = model.fit_offline(examples)
    serialized_model = model.to_json()
    _write_text(arguments.output, serialized_model + "\n")
    return {
        "status": "ok",
        "command": "train",
        "datasetFingerprint": dataset.fingerprint,
        "modelArtifactVersion": MODEL_ARTIFACT_VERSION,
        "modelFingerprint": _fingerprint(serialized_model),
        "trainingExampleCount": metadata.training_example_count,
        "output": str(arguments.output),
    }


# _evaluate compares paired synthetic outcomes and writes JSON and Markdown reports.
def _evaluate(arguments: Namespace) -> dict[str, Any]:
    if arguments.evaluation_version != EVALUATION_SCHEMA_VERSION:
        raise ValueError("unsupported evaluation version")
    dataset = _read_dataset(arguments.dataset)
    serialized_model = arguments.model.read_text(encoding="utf-8")
    model = LinUCBBandit.from_json(serialized_model)
    samples = _evaluation_samples(dataset, model)
    thresholds = EvaluationThresholds(
        minimum_reward_delta=arguments.minimum_reward_delta,
        maximum_dispute_rate_delta=arguments.maximum_dispute_rate_delta,
    )
    report = evaluate_paired_samples(samples, thresholds)
    model_fingerprint = _fingerprint(serialized_model.strip())
    publication = build_evaluation_publication(
        dataset_version=dataset.schema_version,
        dataset_seed=dataset.seed,
        dataset_fingerprint=dataset.fingerprint,
        feature_version=FEATURE_VERSION_V1,
        reward_config_version=REWARD_CONFIG_VERSION,
        baseline_strategy=BASELINE_STRATEGY_NAME,
        baseline_model_version=BASELINE_MODEL_VERSION,
        candidate_strategy=BANDIT_STRATEGY_NAME,
        candidate_model_version=BANDIT_MODEL_VERSION,
        model_artifact_version=MODEL_ARTIFACT_VERSION,
        model_fingerprint=model_fingerprint,
        evaluation=report,
    )
    _write_text(arguments.report_json, publication.to_json() + "\n")
    _write_text(arguments.report_markdown, publication.to_markdown())
    if not report.passes_thresholds:
        raise ValueError(
            "evaluation regression: " + ", ".join(report.regressions)
        )
    return {
        "status": "ok",
        "command": "evaluate",
        "datasetFingerprint": dataset.fingerprint,
        "modelFingerprint": model_fingerprint,
        "evaluationVersion": report.schema_version,
        "reportFingerprint": _fingerprint(publication.to_json()),
        "reportJson": str(arguments.report_json),
        "reportMarkdown": str(arguments.report_markdown),
    }


# _serve starts a bounded local HTTP adapter for recommendation requests.
def _serve(arguments: Namespace) -> None:
    model = LinUCBBandit.from_json(arguments.model.read_text(encoding="utf-8"))
    service = RecommendationService(model)
    request_handler = _request_handler(service)
    server = ThreadingHTTPServer((arguments.host, arguments.port), request_handler)
    print(
        json.dumps(
            {
                "status": "serving",
                "host": arguments.host,
                "port": arguments.port,
                "modelFingerprint": _fingerprint(model.to_json()),
            },
            sort_keys=True,
        )
    )
    server.serve_forever()


# _request_handler builds an HTTP handler bound to one immutable service object.
def _request_handler(
    service: RecommendationService,
) -> type[BaseHTTPRequestHandler]:
    class RequestHandler(BaseHTTPRequestHandler):
        # do_GET serves only bounded health and model-metadata operations.
        def do_GET(self) -> None:
            if self.path == "/health":
                self._write_json(HTTPStatus.OK, service.health())
                return
            if self.path == "/metadata":
                self._write_json(HTTPStatus.OK, service.model_metadata())
                return
            self._write_json(HTTPStatus.NOT_FOUND, {"code": "not_found"})

        # do_POST serves only the recommendation operation.
        def do_POST(self) -> None:
            if self.path != "/recommend":
                self._write_json(HTTPStatus.NOT_FOUND, {"code": "not_found"})
                return
            content_length = _content_length(self.headers.get("Content-Length"))
            if content_length > service.maximum_request_bytes:
                self._write_json(
                    HTTPStatus.REQUEST_ENTITY_TOO_LARGE,
                    {"code": "request_too_large"},
                )
                return
            request_body = self.rfile.read(content_length)
            try:
                response_body = service.recommend(request_body)
            except ServiceError as error:
                self._write_json(
                    HTTPStatus.BAD_REQUEST,
                    {"code": error.code, "message": str(error)},
                )
                return
            self.send_response(HTTPStatus.OK)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response_body)))
            self.end_headers()
            self.wfile.write(response_body)

        # _write_json emits one compact response without logging request bodies.
        def _write_json(self, status: HTTPStatus, response: dict[str, str]) -> None:
            response_body = json.dumps(
                response,
                sort_keys=True,
                separators=(",", ":"),
            ).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response_body)))
            self.end_headers()
            self.wfile.write(response_body)

        # log_message disables default logging that could expose request paths.
        def log_message(self, format_string: str, *arguments: object) -> None:
            return

    return RequestHandler


# _training_examples extracts only selected offers with complete validated outcomes.
def _training_examples(
    dataset: SyntheticDataset,
    reward_config: RewardConfig,
) -> tuple[OfflineTrainingExample, ...]:
    examples: list[OfflineTrainingExample] = []
    for scenario in dataset.scenarios:
        reward = calculate_reward(scenario.outcome_events, reward_config)
        feature_batch = build_feature_batch(scenario.context)
        feature_index = feature_batch.candidate_ids.index(reward.offer_id)
        examples.append(
            OfflineTrainingExample(
                event_id=scenario.outcome_events[-1].event_id,
                feature_version=feature_batch.feature_version,
                features=feature_batch.rows[feature_index],
                reward=reward.total,
            )
        )
    return tuple(examples)


# _evaluation_samples simulates paired outcomes for both strategies on each scenario.
def _evaluation_samples(
    dataset: SyntheticDataset,
    model: LinUCBBandit,
) -> tuple[PairedEvaluationSample, ...]:
    samples: list[PairedEvaluationSample] = []
    reward_config = RewardConfig.default()
    for scenario_index, scenario in enumerate(dataset.scenarios):
        feature_batch = build_feature_batch(scenario.context)
        candidate_recommendation = model.rank(scenario.context, feature_batch)
        baseline_offer_id = scenario.recommendation.ranked_offer_ids[0]
        candidate_offer_id = candidate_recommendation.ranked_offer_ids[0]
        samples.append(
            PairedEvaluationSample(
                sample_id=f"evaluation_{scenario_index:05d}",
                segment=scenario.segment,
                synthetic=True,
                baseline=_simulated_strategy_outcome(
                    scenario.context,
                    baseline_offer_id,
                    baseline_offer_id,
                    reward_config,
                ),
                candidate=_simulated_strategy_outcome(
                    scenario.context,
                    candidate_offer_id,
                    baseline_offer_id,
                    reward_config,
                ),
                candidate_outcome_observed=True,
            )
        )
    return tuple(samples)


# _simulated_strategy_outcome derives deterministic synthetic business outcomes.
def _simulated_strategy_outcome(
    context: DecisionContext,
    selected_offer_id: str,
    preferred_offer_id: str,
    reward_config: RewardConfig,
) -> StrategyOutcome:
    selected_offer = next(
        candidate
        for candidate in context.candidates
        if candidate.offer_id == selected_offer_id
    )
    accepted = selected_offer_id == preferred_offer_id
    fulfilled = selected_offer.historical_delivery_rate_bps >= 9000
    disputed = fulfilled and selected_offer.historical_dispute_rate_bps >= 800
    choice_reward = reward_config.accepted if accepted else reward_config.overridden
    terminal_reward = reward_config.fulfilled if fulfilled else reward_config.failed
    dispute_reward = reward_config.disputed if disputed else 0.0
    return StrategyOutcome(
        accepted=accepted,
        amount_atomic=int(selected_offer.amount),
        fulfilled=fulfilled,
        disputed=disputed,
        reward=choice_reward + terminal_reward + dispute_reward,
    )


# _read_dataset loads and validates one explicit dataset path.
def _read_dataset(dataset_path: Path) -> SyntheticDataset:
    return SyntheticDataset.from_json(dataset_path.read_text(encoding="utf-8"))


# _write_text creates parent directories and writes one UTF-8 artifact.
def _write_text(output_path: Path, contents: str) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(contents, encoding="utf-8")


# _fingerprint returns a lowercase SHA-256 digest for command output.
def _fingerprint(contents: str) -> str:
    return hashlib.sha256(contents.encode("utf-8")).hexdigest()


# _content_length validates the required HTTP request length header.
def _content_length(raw_content_length: str | None) -> int:
    if raw_content_length is None:
        raise ServiceError("invalid_request", "Content-Length is required")
    try:
        content_length = int(raw_content_length)
    except ValueError:
        raise ServiceError("invalid_request", "Content-Length is invalid") from None
    if content_length < 0:
        raise ServiceError("invalid_request", "Content-Length is invalid")
    return content_length


if __name__ == "__main__":
    raise SystemExit(main())
