"""AgentPay seller-request verification for ASGI applications."""

import asyncio
import base64
import hashlib
import hmac
import json
import threading
from datetime import datetime, timedelta, timezone
from typing import Awaitable, Callable, Mapping, Protocol

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import encode_dss_signature

SIGNATURE_DOMAIN = "agentpay.seller-request.v1"
DEFAULT_MAXIMUM_AGE = timedelta(minutes=5)
DEFAULT_MAXIMUM_BODY_BYTES = 1024 * 1024
EXECUTION_CAPABILITY_TYPE = "agentpay-execution+jwt"
MINIMUM_EXECUTION_LIFETIME_SECONDS = 30
MAXIMUM_EXECUTION_LIFETIME_SECONDS = 60
MAXIMUM_EXECUTION_TOKEN_BYTES = 16 * 1024

class VerificationError(Exception):
    """Reports a stable machine-readable verification failure."""


class ReplayStore(Protocol):
    """Atomically claims accepted transaction identifiers."""

    async def claim(self, transaction_id: str, timestamp: datetime) -> bool:
        """Returns true only for the first accepted transaction identifier."""


class SyncReplayStore(Protocol):
    """Atomically claims accepted transaction identifiers for WSGI applications."""

    def claim(self, transaction_id: str, timestamp: datetime) -> bool:
        """Returns true only for the first accepted transaction identifier."""


class ExecutionReplayStore(Protocol):
    """Atomically consumes execution-capability identifiers."""

    async def claim(self, jti: str, expires_at: datetime) -> bool:
        """Returns true only for the first accepted capability identifier."""


class ExecutionKeyResolver(Protocol):
    """Resolves one AgentPay ES256 public JWK by key identifier."""

    async def __call__(self, key_id: str) -> Mapping[str, str]:
        """Returns the public JWK for key_id."""


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


class MemorySyncReplayStore:
    """Provides process-local replay protection for WSGI tests and development."""

    def __init__(self) -> None:
        """Creates an empty synchronous replay store."""
        self._transactions: set[str] = set()
        self._lock = threading.Lock()

    def claim(self, transaction_id: str, timestamp: datetime) -> bool:
        """Claims a transaction once within the current process."""
        del timestamp
        with self._lock:
            if transaction_id in self._transactions:
                return False
            self._transactions.add(transaction_id)
            return True


class MemoryExecutionReplayStore:
    """Provides process-local execution-JTI replay protection for tests."""

    def __init__(self) -> None:
        self._identifiers: set[str] = set()
        self._lock = asyncio.Lock()

    async def claim(self, jti: str, expires_at: datetime) -> bool:
        del expires_at
        async with self._lock:
            if jti in self._identifiers:
                return False
            self._identifiers.add(jti)
            return True


async def verify_execution_request(
    *,
    issuer: str,
    seller_id: str,
    route_id: str,
    method: str,
    path: str,
    body: bytes,
    headers: Mapping[str, str],
    key_resolver: ExecutionKeyResolver,
    replay_store: ExecutionReplayStore,
    now: datetime | None = None,
) -> dict:
    """Verifies an exact ES256 execution capability and consumes its JTI."""
    raw_token = headers.get("x-agentpay-execution-capability")
    transaction_header = headers.get("x-agentpay-transaction-id")
    if (
        not raw_token
        or not transaction_header
        or len(raw_token.encode()) > MAXIMUM_EXECUTION_TOKEN_BYTES
    ):
        raise VerificationError("invalid_capability")
    segments = raw_token.split(".")
    if len(segments) != 3:
        raise VerificationError("invalid_capability")
    try:
        header = json.loads(_decode_base64url(segments[0]))
        claims = json.loads(_decode_base64url(segments[1]))
        signature = _decode_base64url(segments[2])
    except (ValueError, TypeError, json.JSONDecodeError):
        raise VerificationError("invalid_capability") from None
    if (
        not isinstance(header, dict)
        or header.get("typ") != EXECUTION_CAPABILITY_TYPE
        or header.get("alg") != "ES256"
        or not isinstance(header.get("kid"), str)
        or len(signature) != 64
    ):
        raise VerificationError("invalid_capability")
    try:
        jwk = await key_resolver(header["kid"])
    except Exception:
        raise VerificationError("key_unavailable") from None
    if not _verify_es256(
        f"{segments[0]}.{segments[1]}".encode(), signature, jwk
    ):
        raise VerificationError("invalid_capability")
    if not _valid_execution_claims(claims):
        raise VerificationError("invalid_capability")
    current_time = (now or datetime.now(timezone.utc)).astimezone(timezone.utc)
    now_seconds = int(current_time.timestamp())
    lifetime = claims["exp"] - claims["iat"]
    if (
        claims["iss"] != issuer.rstrip("/")
        or claims["aud"] != f"urn:agentpay:seller:{seller_id}"
        or claims["iat"] > now_seconds
        or now_seconds >= claims["exp"]
        or lifetime < MINIMUM_EXECUTION_LIFETIME_SECONDS
        or lifetime > MAXIMUM_EXECUTION_LIFETIME_SECONDS
    ):
        raise VerificationError("invalid_capability")
    body_hash = hashlib.sha256(body).hexdigest()
    if (
        claims["sub"] != transaction_header
        or claims["transactionId"] != transaction_header
        or claims["sellerId"] != seller_id
        or claims["routeId"] != route_id
        or claims["method"] != method.upper()
        or claims["path"] != path
        or not hmac.compare_digest(claims["bodySha256"], body_hash)
        or claims["paymentFinality"] != "finalized"
    ):
        raise VerificationError("binding_mismatch")
    expires_at = datetime.fromtimestamp(claims["exp"], timezone.utc)
    if not await replay_store.claim(claims["jti"], expires_at):
        raise VerificationError("replay")
    return claims


def _valid_execution_claims(value: object) -> bool:
    if not isinstance(value, dict):
        return False
    string_fields = (
        "iss", "aud", "sub", "sellerId", "routeId", "transactionId",
        "method", "path", "bodySha256", "paymentFinality", "jti",
    )
    return (
        all(isinstance(value.get(field), str) and value[field] for field in string_fields)
        and isinstance(value.get("iat"), int)
        and not isinstance(value.get("iat"), bool)
        and isinstance(value.get("exp"), int)
        and not isinstance(value.get("exp"), bool)
        and value["jti"].startswith("xec_")
        and len(value["bodySha256"]) == 64
        and all(character in "0123456789abcdef" for character in value["bodySha256"])
    )


def _decode_base64url(value: str) -> bytes:
    if not value or any(character not in "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_" for character in value):
        raise ValueError("invalid base64url")
    padding = "=" * (-len(value) % 4)
    return base64.b64decode(value + padding, altchars=b"-_", validate=True)


def _verify_es256(signing_input: bytes, signature: bytes, jwk: Mapping[str, str]) -> bool:
    try:
        if jwk.get("kty") != "EC" or jwk.get("crv") != "P-256" or jwk.get("alg") not in (None, "ES256"):
            return False
        public_key = ec.EllipticCurvePublicNumbers(
            int.from_bytes(_decode_base64url(jwk["x"]), "big"),
            int.from_bytes(_decode_base64url(jwk["y"]), "big"),
            ec.SECP256R1(),
        ).public_key()
        r = int.from_bytes(signature[:32], "big")
        s = int.from_bytes(signature[32:], "big")
        public_key.verify(
            encode_dss_signature(r, s),
            signing_input,
            ec.ECDSA(hashes.SHA256()),
        )
        return True
    except (InvalidSignature, KeyError, TypeError, ValueError):
        return False


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
    transaction_id, timestamp = _validate_request(
        secret,
        method,
        path,
        body,
        headers,
        now,
        maximum_age,
    )
    if not await replay_store.claim(transaction_id, timestamp):
        raise VerificationError("replay")


def _validate_request(
    secret: bytes,
    method: str,
    path: str,
    body: bytes,
    headers: Mapping[str, str],
    now: datetime | None,
    maximum_age: timedelta,
) -> tuple[str, datetime]:
    """Validates signed request facts before a replay store is selected."""
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
    return transaction_id, timestamp


def verify_request_sync(
    secret: bytes,
    method: str,
    path: str,
    body: bytes,
    headers: Mapping[str, str],
    replay_store: SyncReplayStore,
    now: datetime | None = None,
    maximum_age: timedelta = DEFAULT_MAXIMUM_AGE,
) -> None:
    """Verifies one WSGI request and atomically blocks replay."""
    transaction_id, timestamp = _validate_request(
        secret,
        method,
        path,
        body,
        headers,
        now,
        maximum_age,
    )
    if not replay_store.claim(transaction_id, timestamp):
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
