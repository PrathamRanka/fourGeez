"""Expose reproducible local commands for simulation, training, and evaluation.

TODO(RL-009):
- Add `simulate`, `train`, `evaluate`, and `serve` subcommands.
- Require explicit input/output paths and configuration versions.
- Print dataset/model fingerprints in every command result.
- Exit non-zero on validation failure or regression against configured thresholds.
- Keep exploratory notebook behavior out of this production entry point.
"""


def main() -> None:
    """Fail clearly until the RL teammate implements the command surface."""
    raise SystemExit("AgentPay RL workspace is scaffolded; implementation has not started.")
