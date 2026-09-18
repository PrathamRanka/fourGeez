import base64
import hashlib
import hmac
import json
import unittest
from datetime import datetime, timezone

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import decode_dss_signature

from agentpay_verify import (
    AgentPayASGIMiddleware,
    MemoryExecutionReplayStore,
    MemoryReplayStore,
    MemorySyncReplayStore,
    VerificationError,
    verify_request,
    verify_execution_request,
    verify_request_sync,
)


SECRET = b"0123456789abcdef0123456789abcdef"
NOW = datetime(2026, 9, 17, 10, 0, 0, tzinfo=timezone.utc)


# sign creates an independent request signature fixture.
def sign(body: bytes, timestamp: str, transaction_id: str) -> str:
    body_hash = hashlib.sha256(body).hexdigest()
    canonical = "\n".join(
        [
            "agentpay.seller-request.v1",
            timestamp,
            "POST",
            "/fulfill",
            body_hash,
            transaction_id,
        ]
    )
    digest = hmac.new(SECRET, canonical.encode(), hashlib.sha256).digest()
    return base64.b64encode(digest).decode()


class VerificationTests(unittest.IsolatedAsyncioTestCase):
    # test_verifies_and_blocks_replay covers the successful and duplicate paths.
    async def test_verifies_and_blocks_replay(self) -> None:
        timestamp = "2026-09-17T10:00:00Z"
        headers = {
            "x-agentpay-signature": sign(b"hello", timestamp, "txn_python"),
            "x-agentpay-timestamp": timestamp,
            "x-agentpay-transaction-id": "txn_python",
        }
        replay_store = MemoryReplayStore()
        await verify_request(
            SECRET, "POST", "/fulfill", b"hello", headers, replay_store, NOW
        )
        with self.assertRaisesRegex(VerificationError, "replay"):
            await verify_request(
                SECRET, "POST", "/fulfill", b"hello", headers, replay_store, NOW
            )

    # test_rejects_modified_body verifies exact raw-body binding.
    async def test_rejects_modified_body(self) -> None:
        timestamp = "2026-09-17T10:00:00Z"
        headers = {
            "x-agentpay-signature": sign(b"hello", timestamp, "txn_modified"),
            "x-agentpay-timestamp": timestamp,
            "x-agentpay-transaction-id": "txn_modified",
        }
        with self.assertRaisesRegex(VerificationError, "invalid_signature"):
            await verify_request(
                SECRET,
                "POST",
                "/fulfill",
                b"changed",
                headers,
                MemoryReplayStore(),
                NOW,
            )

    # test_preserves_fractional_timestamp verifies literal canonicalization.
    async def test_preserves_fractional_timestamp(self) -> None:
        timestamp = "2026-09-17T10:00:00.123456Z"
        headers = {
            "x-agentpay-signature": sign(b"hello", timestamp, "txn_fractional"),
            "x-agentpay-timestamp": timestamp,
            "x-agentpay-transaction-id": "txn_fractional",
        }
        await verify_request(
            SECRET,
            "POST",
            "/fulfill",
            b"hello",
            headers,
            MemoryReplayStore(),
            NOW,
        )

    # test_asgi_middleware_verifies_and_replays_body covers framework integration.
    async def test_asgi_middleware_verifies_and_replays_body(self) -> None:
        timestamp = "2026-09-17T10:00:00Z"
        sent_messages: list[dict] = []

        async def application(scope, receive, send) -> None:
            body_message = await receive()
            self.assertEqual(body_message["body"], b"hello")
            await send({"type": "http.response.start", "status": 204, "headers": []})
            await send({"type": "http.response.body", "body": b""})

        middleware = AgentPayASGIMiddleware(
            application,
            SECRET,
            MemoryReplayStore(),
            clock=lambda: NOW,
        )
        scope = {
            "type": "http",
            "method": "POST",
            "path": "/fulfill",
            "headers": [
                (b"x-agentpay-signature", sign(b"hello", timestamp, "txn_asgi").encode()),
                (b"x-agentpay-timestamp", timestamp.encode()),
                (b"x-agentpay-transaction-id", b"txn_asgi"),
            ],
        }

        messages = [{"type": "http.request", "body": b"hello", "more_body": False}]

        async def receive():
            return messages.pop(0)

        async def send(message) -> None:
            sent_messages.append(message)

        await middleware(scope, receive, send)
        self.assertEqual(sent_messages[0]["status"], 204)

        sent_messages.clear()
        messages.append({"type": "http.request", "body": b"hello", "more_body": False})
        await middleware(scope, receive, send)
        self.assertEqual(sent_messages[0]["status"], 409)

    async def test_execution_capability_binds_request_and_consumes_jti(self) -> None:
        body = b'{"topic":"payments"}'
        token, jwk = sign_execution_capability(body)

        async def resolve_key(key_id: str) -> dict:
            self.assertEqual(key_id, "key-1")
            return jwk

        replay_store = MemoryExecutionReplayStore()
        claims = await verify_execution_request(
            issuer="https://api.agentpay.test",
            seller_id="sel_test",
            route_id="rte_test",
            method="POST",
            path="/research",
            body=body,
            headers={
                "x-agentpay-execution-capability": token,
                "x-agentpay-transaction-id": "txn_test",
            },
            key_resolver=resolve_key,
            replay_store=replay_store,
            now=NOW,
        )
        self.assertEqual(claims["transactionId"], "txn_test")
        with self.assertRaisesRegex(VerificationError, "replay"):
            await verify_execution_request(
                issuer="https://api.agentpay.test",
                seller_id="sel_test",
                route_id="rte_test",
                method="POST",
                path="/research",
                body=body,
                headers={
                    "x-agentpay-execution-capability": token,
                    "x-agentpay-transaction-id": "txn_test",
                },
                key_resolver=resolve_key,
                replay_store=replay_store,
                now=NOW,
            )

    async def test_execution_capability_rejects_modified_body(self) -> None:
        token, jwk = sign_execution_capability(b'{"topic":"payments"}')

        async def resolve_key(_key_id: str) -> dict:
            return jwk

        with self.assertRaisesRegex(VerificationError, "binding_mismatch"):
            await verify_execution_request(
                issuer="https://api.agentpay.test",
                seller_id="sel_test",
                route_id="rte_test",
                method="POST",
                path="/research",
                body=b'{"topic":"changed"}',
                headers={
                    "x-agentpay-execution-capability": token,
                    "x-agentpay-transaction-id": "txn_test",
                },
                key_resolver=resolve_key,
                replay_store=MemoryExecutionReplayStore(),
                now=NOW,
            )


class SyncVerificationTests(unittest.TestCase):
    # test_sync_verifier_supports_wsgi_frameworks covers Flask and Django adapters.
    def test_sync_verifier_supports_wsgi_frameworks(self) -> None:
        timestamp = "2026-09-17T10:00:00Z"
        headers = {
            "x-agentpay-signature": sign(b"hello", timestamp, "txn_wsgi"),
            "x-agentpay-timestamp": timestamp,
            "x-agentpay-transaction-id": "txn_wsgi",
        }
        replay_store = MemorySyncReplayStore()
        verify_request_sync(
            SECRET, "POST", "/fulfill", b"hello", headers, replay_store, NOW
        )
        with self.assertRaisesRegex(VerificationError, "replay"):
            verify_request_sync(
                SECRET, "POST", "/fulfill", b"hello", headers, replay_store, NOW
            )


def _b64url(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()


def sign_execution_capability(body: bytes) -> tuple[str, dict]:
    issued_at = int(NOW.timestamp())
    header = {"typ": "agentpay-execution+jwt", "alg": "ES256", "kid": "key-1"}
    claims = {
        "iss": "https://api.agentpay.test",
        "aud": "urn:agentpay:seller:sel_test",
        "sub": "txn_test",
        "sellerId": "sel_test",
        "routeId": "rte_test",
        "transactionId": "txn_test",
        "method": "POST",
        "path": "/research",
        "bodySha256": hashlib.sha256(body).hexdigest(),
        "paymentFinality": "finalized",
        "jti": "xec_test",
        "iat": issued_at,
        "exp": issued_at + 45,
    }
    signing_input = ".".join(
        _b64url(json.dumps(value, separators=(",", ":")).encode())
        for value in (header, claims)
    )
    private_key = ec.generate_private_key(ec.SECP256R1())
    der_signature = private_key.sign(
        signing_input.encode(), ec.ECDSA(hashes.SHA256())
    )
    r, s = decode_dss_signature(der_signature)
    signature = r.to_bytes(32, "big") + s.to_bytes(32, "big")
    public_numbers = private_key.public_key().public_numbers()
    jwk = {
        "kty": "EC",
        "crv": "P-256",
        "alg": "ES256",
        "use": "sig",
        "kid": "key-1",
        "x": _b64url(public_numbers.x.to_bytes(32, "big")),
        "y": _b64url(public_numbers.y.to_bytes(32, "big")),
    }
    return f"{signing_input}.{_b64url(signature)}", jwk


if __name__ == "__main__":
    unittest.main()
