"""Versioned and auditable reward construction from completed outcomes."""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import UTC, datetime

from .contracts import OutcomeEvent, OutcomeType

REWARD_CONFIG_VERSION = "reward-v1"
MAXIMUM_ABSOLUTE_COMPONENT = 100.0


class RewardError(ValueError):
    """Reports an invalid reward configuration or event group."""


class IncompleteOutcomeSequence(RewardError):
    """Reports events that cannot safely produce an offline reward."""


@dataclass(frozen=True)
class RewardConfig:
    """Defines explicit weights for every supported outcome type."""

    version: str
    accepted: float
    overridden: float
    fulfilled: float
    failed: float
    disputed: float
    resolved: float

    # __post_init__ rejects unknown versions and unbounded component values.
    def __post_init__(self) -> None:
        if self.version != REWARD_CONFIG_VERSION:
            raise RewardError(f"unsupported reward config version: {self.version}")
        for field_name, value in self._weights().items():
            if not math.isfinite(value):
                raise RewardError(f"{field_name} reward must be finite")
            if abs(value) > MAXIMUM_ABSOLUTE_COMPONENT:
                raise RewardError(
                    f"{field_name} reward exceeds {MAXIMUM_ABSOLUTE_COMPONENT}"
                )

    # default returns the documented version-one reward policy.
    @classmethod
    def default(cls) -> RewardConfig:
        return cls(
            version=REWARD_CONFIG_VERSION,
            accepted=0.1,
            overridden=-0.1,
            fulfilled=1.0,
            failed=-1.0,
            disputed=-0.5,
            resolved=0.0,
        )

    # value_for maps one allowlisted outcome to its configured component.
    def value_for(self, event_type: OutcomeType) -> float:
        return self._weights()[event_type]

    # _weights exposes a complete event-to-weight mapping for validation.
    def _weights(self) -> dict[OutcomeType, float]:
        return {
            OutcomeType.RECOMMENDATION_ACCEPTED: self.accepted,
            OutcomeType.RECOMMENDATION_OVERRIDDEN: self.overridden,
            OutcomeType.TRANSACTION_FULFILLED: self.fulfilled,
            OutcomeType.TRANSACTION_FAILED: self.failed,
            OutcomeType.TRANSACTION_DISPUTED: self.disputed,
            OutcomeType.DISPUTE_RESOLVED: self.resolved,
        }


@dataclass(frozen=True)
class RewardComponent:
    """Records the exact event contribution to an aggregate reward."""

    event_id: str
    event_type: OutcomeType
    value: float


@dataclass(frozen=True)
class RewardResult:
    """Contains the aggregate reward and every auditable component."""

    config_version: str
    recommendation_id: str
    offer_id: str
    total: float
    components: tuple[RewardComponent, ...]


# calculate_reward validates a completed event sequence before scoring it.
def calculate_reward(
    events: tuple[OutcomeEvent, ...],
    config: RewardConfig,
    as_of: str | None = None,
) -> RewardResult:
    _validate_outcome_sequence(events, as_of)
    components = tuple(
        RewardComponent(
            event_id=event.event_id,
            event_type=event.event_type,
            value=config.value_for(event.event_type),
        )
        for event in events
    )
    total = round(sum(component.value for component in components), 12)
    first_event = events[0]
    return RewardResult(
        config_version=config.version,
        recommendation_id=first_event.recommendation_id,
        offer_id=first_event.offer_id,
        total=total,
        components=components,
    )


# _validate_outcome_sequence rejects incomplete, mixed, or future event groups.
def _validate_outcome_sequence(
    events: tuple[OutcomeEvent, ...],
    as_of: str | None,
) -> None:
    if len(events) < 2:
        raise IncompleteOutcomeSequence("outcomes require a choice and terminal transaction event")

    first_event = events[0]
    _validate_shared_identity(events, first_event)
    timestamps = tuple(_parse_utc(event.occurred_at) for event in events)
    if timestamps != tuple(sorted(timestamps)) or len(set(timestamps)) != len(timestamps):
        raise IncompleteOutcomeSequence("outcomes must be in strictly increasing time order")

    if as_of is not None:
        cutoff = _parse_utc(as_of)
        if any(timestamp > cutoff for timestamp in timestamps):
            raise IncompleteOutcomeSequence("outcome occurs after the evaluation cutoff")

    event_types = tuple(event.event_type for event in events)
    _validate_event_types(event_types)


# _validate_shared_identity prevents unrelated outcomes from sharing one reward.
def _validate_shared_identity(
    events: tuple[OutcomeEvent, ...],
    first_event: OutcomeEvent,
) -> None:
    for event in events[1:]:
        if event.recommendation_id != first_event.recommendation_id:
            raise IncompleteOutcomeSequence("events must share one recommendation")
        if event.offer_id != first_event.offer_id:
            raise IncompleteOutcomeSequence("events must share one offer")
        if event.feature_version != first_event.feature_version:
            raise IncompleteOutcomeSequence("events must share one feature version")
        if event.strategy_version != first_event.strategy_version:
            raise IncompleteOutcomeSequence("events must share one strategy version")
        if event.model_version != first_event.model_version:
            raise IncompleteOutcomeSequence("events must share one model version")


# _validate_event_types enforces one complete chronological outcome lifecycle.
def _validate_event_types(event_types: tuple[OutcomeType, ...]) -> None:
    choice_events = {
        OutcomeType.RECOMMENDATION_ACCEPTED,
        OutcomeType.RECOMMENDATION_OVERRIDDEN,
    }
    if event_types[0] not in choice_events:
        raise IncompleteOutcomeSequence("first outcome must record acceptance or override")

    terminal_events = {
        OutcomeType.TRANSACTION_FULFILLED,
        OutcomeType.TRANSACTION_FAILED,
    }
    if event_types[1] not in terminal_events:
        raise IncompleteOutcomeSequence("second outcome must record fulfillment or failure")

    if event_types[1] == OutcomeType.TRANSACTION_FAILED and len(event_types) != 2:
        raise IncompleteOutcomeSequence("failed transactions cannot contain later outcomes")

    allowed_fulfilled_suffixes = (
        (),
        (OutcomeType.TRANSACTION_DISPUTED,),
        (
            OutcomeType.TRANSACTION_DISPUTED,
            OutcomeType.DISPUTE_RESOLVED,
        ),
    )
    if event_types[1] == OutcomeType.TRANSACTION_FULFILLED:
        suffix = event_types[2:]
        if suffix not in allowed_fulfilled_suffixes:
            raise IncompleteOutcomeSequence("fulfilled outcome sequence is invalid")


# _parse_utc parses strict UTC timestamps used by validated outcome contracts.
def _parse_utc(value: str) -> datetime:
    parsed = datetime.fromisoformat(value)
    if parsed.tzinfo != UTC:
        raise IncompleteOutcomeSequence("outcome timestamp must use UTC")
    return parsed
