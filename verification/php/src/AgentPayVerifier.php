<?php

declare(strict_types=1);

namespace AgentPay\Verify;

use DateTimeImmutable;
use DateTimeInterface;
use DateTimeZone;
use RuntimeException;

/** Atomically claims accepted transaction identifiers. */
interface ReplayStore
{
    /** Returns true only for the first accepted transaction identifier. */
    public function claim(string $transactionId, DateTimeInterface $timestamp): bool;
}

/** Provides process-local replay protection for tests and development. */
final class MemoryReplayStore implements ReplayStore
{
    /** @var array<string, true> */
    private array $transactionIds = [];

    /** Claims one transaction identifier exactly once. */
    public function claim(string $transactionId, DateTimeInterface $timestamp): bool
    {
        unset($timestamp);
        if (isset($this->transactionIds[$transactionId])) {
            return false;
        }
        $this->transactionIds[$transactionId] = true;
        return true;
    }
}

/** Reports a stable machine-readable verification failure. */
final class VerificationException extends RuntimeException
{
}

/** Verifies AgentPay signatures for Laravel and other PHP HTTP stacks. */
final class AgentPayVerifier
{
    private const SIGNATURE_DOMAIN = 'agentpay.seller-request.v1';
    private const MAXIMUM_AGE_SECONDS = 300;

    /** Creates a verifier with explicit replay storage. */
    public function __construct(
        private readonly string $secret,
        private readonly ReplayStore $replayStore,
    ) {
        if (strlen($secret) < 32) {
            throw new RuntimeException('AgentPay verification secret must contain at least 32 bytes');
        }
    }

    /** Verifies one exact raw request and claims its transaction identifier. */
    public function verify(
        string $method,
        string $path,
        string $body,
        array $headers,
        ?DateTimeImmutable $now = null,
    ): void {
        $signatureValue = self::header($headers, 'x-agentpay-signature');
        $timestampValue = self::header($headers, 'x-agentpay-timestamp');
        $transactionId = self::header($headers, 'x-agentpay-transaction-id');
        if ($signatureValue === null || $timestampValue === null || $transactionId === null || trim($transactionId) === '') {
            throw new VerificationException('invalid_signature');
        }
        try {
            $timestamp = new DateTimeImmutable($timestampValue);
        } catch (\Exception) {
            throw new VerificationException('invalid_signature');
        }
        $currentTime = $now ?? new DateTimeImmutable('now', new DateTimeZone('UTC'));
        if (abs($currentTime->getTimestamp() - $timestamp->getTimestamp()) > self::MAXIMUM_AGE_SECONDS) {
            throw new VerificationException('stale_request');
        }
        $providedSignature = base64_decode($signatureValue, true);
        if ($providedSignature === false) {
            throw new VerificationException('invalid_signature');
        }
        $canonical = implode("\n", [
            self::SIGNATURE_DOMAIN,
            $timestampValue,
            strtoupper($method),
            $path,
            hash('sha256', $body),
            $transactionId,
        ]);
        $expectedSignature = hash_hmac('sha256', $canonical, $this->secret, true);
        if (!hash_equals($expectedSignature, $providedSignature)) {
            throw new VerificationException('invalid_signature');
        }
        if (!$this->replayStore->claim($transactionId, $timestamp)) {
            throw new VerificationException('replay');
        }
    }

    /** Finds a header without depending on array key casing. */
    private static function header(array $headers, string $name): ?string
    {
        foreach ($headers as $headerName => $value) {
            if (strcasecmp((string) $headerName, $name) === 0) {
                return (string) $value;
            }
        }
        return null;
    }
}
