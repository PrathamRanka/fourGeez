import base64
import hashlib
import hmac
import unittest
from datetime import datetime, timezone

from agentpay_verify import (
    AgentPayASGIMiddleware,
    MemoryReplayStore,
    MemorySyncReplayStore,
    VerificationError,
    verify_request,
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


if __name__ == "__main__":
    unittest.main()
