<?php

declare(strict_types=1);

namespace AgentPay\Verify;

use DateTimeImmutable;
use Throwable;

/** Verifies Laravel request facts before controller execution. */
final class AgentPayVerificationMiddleware
{
    /** Creates middleware with an explicit verifier. */
    public function __construct(private readonly AgentPayVerifier $verifier)
    {
    }

    /** Verifies a normalized Laravel request and invokes the next callable. */
    public function handle(array $request, callable $next): array
    {
        try {
            $this->verifier->verify(
                (string) ($request['method'] ?? ''),
                (string) ($request['path'] ?? ''),
                (string) ($request['body'] ?? ''),
                (array) ($request['headers'] ?? []),
                $request['now'] ?? new DateTimeImmutable('now'),
            );
            return $next($request);
        } catch (VerificationException $exception) {
            return self::reject($exception->getMessage() === 'replay' ? 409 : 401);
        } catch (Throwable) {
            return self::reject(503);
        }
    }

    /** Returns the shared redacted middleware error response. */
    private static function reject(int $status): array
    {
        return [
            'status' => $status,
            'headers' => ['content-type' => 'application/json'],
            'body' => '{"error":"AgentPay verification failed"}',
        ];
    }
}
