"""AgentPay seller-request verification for ASGI applications."""

import asyncio
import base64
import hashlib
import hmac
from datetime import datetime, timedelta, timezone
from typing import Awaitable, Callable, Mapping, Protocol

SIGNATURE_DOMAIN = "agentpay.seller-request.v1"
DEFAULT_MAXIMUM_AGE = timedelta(minutes=5)
DEFAULT_MAXIMUM_BODY_BYTES = 1024 * 1024


class VerificationError(Exception):
    """Reports a stable machine-readable verification failure."""


class ReplayStore(Protocol):
    """Atomically claims accepted transaction identifiers."""

    async def claim(self, transaction_id: str, timestamp: datetime) -> bool:
        """Returns true only for the first accepted transaction identifier."""


class MemoryReplayStore:
    """Provides process-local replay protection for tests and development."""

    def __init__(self) -> None:
        """Creates an empty replay store."""
        self._transactions: set[str] = set()
        self._lock = asyncio.Lock()

    async def claim(self, transaction_id: str, timestamp: datetime) -> bool:
        """Claims a transaction once within the current process."""
        del timestamp
        async with self._lock:
            if transaction_id in self._transactions:
                return False
            self._transactions.add(transaction_id)
            return True


async def verify_request(
    secret: bytes,
    method: str,
    path: str,
    body: bytes,
    headers: Mapping[str, str],
    replay_store: ReplayStore,
    now: datetime | None = None,
    maximum_age: timedelta = DEFAULT_MAXIMUM_AGE,
) -> None:
    """Verifies one exact raw request and atomically blocks replay."""
    if len(secret) < 32:
        raise VerificationError("invalid_configuration")
    signature_value = headers.get("x-agentpay-signature")
    timestamp_value = headers.get("x-agentpay-timestamp")
    transaction_id = headers.get("x-agentpay-transaction-id")
    if not signature_value or not timestamp_value or not transaction_id:
        raise VerificationError("invalid_signature")
    try:
        timestamp = datetime.fromisoformat(timestamp_value.replace("Z", "+00:00"))
        provided_signature = base64.b64decode(signature_value, validate=True)
    except (ValueError, TypeError):
        raise VerificationError("invalid_signature") from None
    if timestamp.tzinfo is None:
        raise VerificationError("invalid_signature")
    current_time = now or datetime.now(timezone.utc)
    if abs(current_time - timestamp.astimezone(timezone.utc)) > maximum_age:
        raise VerificationError("stale_request")
    body_hash = hashlib.sha256(body).hexdigest()
    canonical = "\n".join(
        [
            SIGNATURE_DOMAIN,
            timestamp_value,
            method.upper(),
            path,
            body_hash,
            transaction_id,
        ]
    )
    expected_signature = hmac.new(
        secret, canonical.encode(), hashlib.sha256
    ).digest()
    if not hmac.compare_digest(provided_signature, expected_signature):
        raise VerificationError("invalid_signature")
    if not await replay_store.claim(transaction_id, timestamp):
        raise VerificationError("replay")


class AgentPayASGIMiddleware:
    """Verifies HTTP requests while preserving the exact body for the ASGI app."""

    def __init__(
        self,
        application: Callable[..., Awaitable[None]],
        secret: bytes,
        replay_store: ReplayStore,
        clock: Callable[[], datetime] | None = None,
        maximum_age: timedelta = DEFAULT_MAXIMUM_AGE,
        maximum_body_bytes: int = DEFAULT_MAXIMUM_BODY_BYTES,
    ) -> None:
        """Stores explicit verification dependencies and limits."""
        self._application = application
        self._secret = secret
        self._replay_store = replay_store
        self._clock = clock or (lambda: datetime.now(timezone.utc))
        self._maximum_age = maximum_age
        self._maximum_body_bytes = maximum_body_bytes

    async def __call__(self, scope, receive, send) -> None:
        """Verifies one HTTP request and replays its body to the application."""
        if scope.get("type") != "http":
            await self._application(scope, receive, send)
            return
        body = bytearray()
        while True:
            message = await receive()
            body.extend(message.get("body", b""))
            if len(body) > self._maximum_body_bytes:
                await self._reject(send, 401)
                return
            if not message.get("more_body", False):
                break
        headers = {
            key.decode("latin-1").lower(): value.decode("latin-1")
            for key, value in scope.get("headers", [])
        }
        try:
            await verify_request(
                self._secret,
                scope.get("method", ""),
                scope.get("path", ""),
                bytes(body),
                headers,
                self._replay_store,
                self._clock(),
                self._maximum_age,
            )
        except VerificationError as error:
            await self._reject(send, 409 if str(error) == "replay" else 401)
            return
        except Exception:
            await self._reject(send, 503)
            return

        delivered = False

        async def replay_receive():
            """Returns the verified body once to the downstream application."""
            nonlocal delivered
            if delivered:
                return {"type": "http.request", "body": b"", "more_body": False}
            delivered = True
            return {"type": "http.request", "body": bytes(body), "more_body": False}

        await self._application(scope, replay_receive, send)

    async def _reject(self, send, status: int) -> None:
        """Returns a redacted verification failure response."""
        response_body = b'{"error":"AgentPay verification failed"}'
        await send(
            {
                "type": "http.response.start",
                "status": status,
                "headers": [(b"content-type", b"application/json")],
            }
        )
        await send({"type": "http.response.body", "body": response_body})
