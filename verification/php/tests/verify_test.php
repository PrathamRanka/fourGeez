<?php

declare(strict_types=1);

require_once __DIR__ . '/../src/AgentPayVerifier.php';
require_once __DIR__ . '/../src/AgentPayVerificationMiddleware.php';

use AgentPay\Verify\AgentPayVerifier;
use AgentPay\Verify\AgentPayVerificationMiddleware;
use AgentPay\Verify\MemoryReplayStore;
use AgentPay\Verify\VerificationException;

const SECRET = '0123456789abcdef0123456789abcdef';
const TIMESTAMP = '2026-09-17T10:00:00Z';

/** Creates an independent known-answer request signature. */
function sign_request(string $body, string $timestamp, string $transactionId): string
{
    $canonical = implode("\n", [
        'agentpay.seller-request.v1',
        $timestamp,
        'POST',
        '/fulfill',
        hash('sha256', $body),
        $transactionId,
    ]);
    return base64_encode(hash_hmac('sha256', $canonical, SECRET, true));
}

/** Verifies that one callback fails with the expected stable error code. */
function assert_rejected(callable $operation, string $expectedCode): void
{
    try {
        $operation();
        throw new RuntimeException($expectedCode . ' request was accepted');
    } catch (VerificationException $exception) {
        if ($exception->getMessage() !== $expectedCode) {
            throw $exception;
        }
    }
}

/** Creates a complete signed request header set. */
function signed_headers(string $body, string $timestamp, string $transactionId): array
{
    return [
        'x-agentpay-signature' => sign_request($body, $timestamp, $transactionId),
        'x-agentpay-timestamp' => $timestamp,
        'x-agentpay-transaction-id' => $transactionId,
    ];
}

$headers = signed_headers('hello', TIMESTAMP, 'txn_php');
$verifier = new AgentPayVerifier(SECRET, new MemoryReplayStore());
$verifier->verify('POST', '/fulfill', 'hello', $headers, new DateTimeImmutable(TIMESTAMP));
assert_rejected(
    fn (): null => $verifier->verify('POST', '/fulfill', 'hello', $headers, new DateTimeImmutable(TIMESTAMP)),
    'replay',
);

$modifiedVerifier = new AgentPayVerifier(SECRET, new MemoryReplayStore());
assert_rejected(
    fn (): null => $modifiedVerifier->verify(
        'POST',
        '/fulfill',
        'changed',
        signed_headers('hello', TIMESTAMP, 'txn_php_changed'),
        new DateTimeImmutable(TIMESTAMP),
    ),
    'invalid_signature',
);

$staleVerifier = new AgentPayVerifier(SECRET, new MemoryReplayStore());
assert_rejected(
    fn (): null => $staleVerifier->verify(
        'POST',
        '/fulfill',
        'hello',
        signed_headers('hello', TIMESTAMP, 'txn_php_stale'),
        new DateTimeImmutable('2026-09-17T10:05:01Z'),
    ),
    'stale_request',
);

try {
    new AgentPayVerifier('short', new MemoryReplayStore());
    throw new RuntimeException('short secret was accepted');
} catch (RuntimeException $exception) {
    if ($exception->getMessage() !== 'AgentPay verification secret must contain at least 32 bytes') {
        throw $exception;
    }
}

$middleware = new AgentPayVerificationMiddleware(
    new AgentPayVerifier(SECRET, new MemoryReplayStore()),
);
$middlewareRequest = [
    'method' => 'POST',
    'path' => '/fulfill',
    'body' => 'hello',
    'headers' => signed_headers('hello', TIMESTAMP, 'txn_php_middleware'),
    'now' => new DateTimeImmutable(TIMESTAMP),
];
$middlewareResponse = $middleware->handle(
    $middlewareRequest,
    fn (array $request): array => [
        'status' => 204,
        'body' => $request['body'],
    ],
);
if ($middlewareResponse !== ['status' => 204, 'body' => 'hello']) {
    throw new RuntimeException('middleware did not preserve the verified request body');
}
