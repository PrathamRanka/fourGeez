"""Tests for reproducible simulation, training, evaluation, and serving commands."""

from __future__ import annotations

import json
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path

from agentpay_rl.bandit import MODEL_ARTIFACT_VERSION
from agentpay_rl.cli import build_parser, main
from agentpay_rl.contracts import FEATURE_VERSION_V1
from agentpay_rl.evaluation import EVALUATION_SCHEMA_VERSION
from agentpay_rl.rewards import REWARD_CONFIG_VERSION
from agentpay_rl.simulator import DATASET_SCHEMA_VERSION


class CliTests(unittest.TestCase):
    # test_pipeline_writes_reproducible_fingerprinted_artifacts verifies commands.
    def test_pipeline_writes_reproducible_fingerprinted_artifacts(self) -> None:
        with tempfile.TemporaryDirectory() as temporary_directory:
            root = Path(temporary_directory)
            dataset_path = root / "dataset.json"
            model_path = root / "model.json"
            report_json_path = root / "report.json"
            report_markdown_path = root / "report.md"

            simulate_result = self._run(
                "simulate",
                "--output",
                str(dataset_path),
                "--dataset-version",
                DATASET_SCHEMA_VERSION,
                "--seed",
                "41",
                "--scenario-count",
                "60",
            )
            train_result = self._run(
                "train",
                "--input",
                str(dataset_path),
                "--output",
                str(model_path),
                "--feature-version",
                FEATURE_VERSION_V1,
                "--reward-version",
                REWARD_CONFIG_VERSION,
                "--seed",
                "41",
            )
            evaluate_result = self._run(
                "evaluate",
                "--dataset",
                str(dataset_path),
                "--model",
                str(model_path),
                "--report-json",
                str(report_json_path),
                "--report-markdown",
                str(report_markdown_path),
                "--evaluation-version",
                EVALUATION_SCHEMA_VERSION,
                "--minimum-reward-delta",
                "-10",
                "--maximum-dispute-rate-delta",
                "1",
            )

            self.assertEqual(simulate_result["datasetVersion"], DATASET_SCHEMA_VERSION)
            self.assertEqual(train_result["modelArtifactVersion"], MODEL_ARTIFACT_VERSION)
            self.assertEqual(evaluate_result["evaluationVersion"], EVALUATION_SCHEMA_VERSION)
            self.assertEqual(
                json.loads(report_json_path.read_text(encoding="utf-8"))["synthetic"],
                True,
            )
            self.assertIn(
                "SYNTHETIC DATA",
                report_markdown_path.read_text(encoding="utf-8"),
            )

    # test_same_simulation_command_produces_same_fingerprint verifies reproducibility.
    def test_same_simulation_command_produces_same_fingerprint(self) -> None:
        with tempfile.TemporaryDirectory() as temporary_directory:
            root = Path(temporary_directory)
            first = self._run(
                "simulate",
                "--output",
                str(root / "first.json"),
                "--dataset-version",
                DATASET_SCHEMA_VERSION,
                "--seed",
                "7",
                "--scenario-count",
                "9",
            )
            second = self._run(
                "simulate",
                "--output",
                str(root / "second.json"),
                "--dataset-version",
                DATASET_SCHEMA_VERSION,
                "--seed",
                "7",
                "--scenario-count",
                "9",
            )

            self.assertEqual(first["datasetFingerprint"], second["datasetFingerprint"])

    # test_invalid_version_exits_nonzero verifies fail-closed command validation.
    def test_invalid_version_exits_nonzero(self) -> None:
        with tempfile.TemporaryDirectory() as temporary_directory:
            exit_code, result = self._invoke(
                "simulate",
                "--output",
                str(Path(temporary_directory) / "dataset.json"),
                "--dataset-version",
                "agentpay.synthetic-dataset.v2",
                "--seed",
                "1",
                "--scenario-count",
                "3",
            )

            self.assertNotEqual(exit_code, 0)
            self.assertEqual(result["status"], "error")

    # test_serve_parser_requires_model_path verifies the serving command exists.
    def test_serve_parser_requires_model_path(self) -> None:
        parsed = build_parser().parse_args(
            (
                "serve",
                "--model",
                "model.json",
                "--host",
                "127.0.0.1",
                "--port",
                "8080",
            )
        )

        self.assertEqual(parsed.command, "serve")
        self.assertEqual(parsed.model, Path("model.json"))

    # _run invokes one successful command and returns its JSON summary.
    def _run(self, *arguments: str) -> dict[str, object]:
        exit_code, result = self._invoke(*arguments)
        self.assertEqual(exit_code, 0, result)
        return result

    # _invoke captures one command's stable JSON result.
    def _invoke(self, *arguments: str) -> tuple[int, dict[str, object]]:
        output = StringIO()
        with redirect_stdout(output):
            exit_code = main(arguments)
        return exit_code, json.loads(output.getvalue())


if __name__ == "__main__":
    unittest.main()
