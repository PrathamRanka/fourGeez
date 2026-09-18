using System.Security.Cryptography;
using System.Text;

namespace AgentPay.Verify;

/// <summary>Atomically claims accepted transaction identifiers.</summary>
public interface IReplayStore
{
    /// <summary>Returns true only for the first accepted transaction identifier.</summary>
    ValueTask<bool> ClaimAsync(string transactionId, DateTimeOffset timestamp, CancellationToken cancellationToken);
}

/// <summary>Provides process-local replay protection for tests and development.</summary>
public sealed class MemoryReplayStore : IReplayStore
{
    private readonly HashSet<string> transactionIds = [];
    private readonly object replayLock = new();

    /// <summary>Claims one transaction identifier exactly once.</summary>
    public ValueTask<bool> ClaimAsync(string transactionId, DateTimeOffset timestamp, CancellationToken cancellationToken)
    {
        cancellationToken.ThrowIfCancellationRequested();
        _ = timestamp;
        lock (replayLock)
        {
            return ValueTask.FromResult(transactionIds.Add(transactionId));
        }
    }
}

/// <summary>Reports a stable request-verification failure.</summary>
public sealed class VerificationException(string code) : Exception(code)
{
    /// <summary>Gets the stable machine-readable error code.</summary>
    public string ErrorCode { get; } = code;
}

/// <summary>Verifies AgentPay request signatures and replay state.</summary>
public sealed class AgentPayVerifier(byte[] secret, IReplayStore replayStore)
{
    private const string SignatureDomain = "agentpay.seller-request.v1";
    private static readonly TimeSpan MaximumAge = TimeSpan.FromMinutes(5);
    private readonly byte[] signingSecret = secret.Length >= 32
        ? secret.ToArray()
        : throw new ArgumentException("AgentPay verification secret must contain at least 32 bytes", nameof(secret));

    /// <summary>Verifies one exact raw request and claims its transaction identifier.</summary>
    public async ValueTask VerifyAsync(
        string method,
        string path,
        ReadOnlyMemory<byte> body,
        IReadOnlyDictionary<string, string> headers,
        DateTimeOffset now,
        CancellationToken cancellationToken = default)
    {
        if (!TryHeader(headers, "x-agentpay-signature", out var signatureValue) ||
            !TryHeader(headers, "x-agentpay-timestamp", out var timestampValue) ||
            !TryHeader(headers, "x-agentpay-transaction-id", out var transactionId) ||
            string.IsNullOrWhiteSpace(transactionId))
        {
            throw new VerificationException("invalid_signature");
        }
        if (!DateTimeOffset.TryParse(timestampValue, out var timestamp) ||
            (now - timestamp).Duration() > MaximumAge)
        {
            throw new VerificationException("stale_request");
        }
        byte[] providedSignature;
        try
        {
            providedSignature = Convert.FromBase64String(signatureValue);
        }
        catch (FormatException)
        {
            throw new VerificationException("invalid_signature");
        }
        var bodyHash = Convert.ToHexString(SHA256.HashData(body.Span)).ToLowerInvariant();
        var canonical = string.Join(
            "\n",
            SignatureDomain,
            timestampValue,
            method.ToUpperInvariant(),
            path,
            bodyHash,
            transactionId);
        var expectedSignature = HMACSHA256.HashData(signingSecret, Encoding.UTF8.GetBytes(canonical));
        if (!CryptographicOperations.FixedTimeEquals(providedSignature, expectedSignature))
        {
            throw new VerificationException("invalid_signature");
        }
        if (!await replayStore.ClaimAsync(transactionId, timestamp, cancellationToken))
        {
            throw new VerificationException("replay");
        }
    }

    /// <summary>Finds a header without depending on dictionary casing.</summary>
    private static bool TryHeader(IReadOnlyDictionary<string, string> headers, string name, out string value)
    {
        foreach (var header in headers)
        {
            if (string.Equals(header.Key, name, StringComparison.OrdinalIgnoreCase))
            {
                value = header.Value;
                return true;
            }
        }
        value = string.Empty;
        return false;
    }
}
