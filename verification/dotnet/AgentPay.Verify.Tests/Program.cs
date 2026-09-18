using System.Security.Cryptography;
using System.Text;
using AgentPay.Verify;
using Microsoft.AspNetCore.Http;

const string Timestamp = "2026-09-17T10:00:00Z";
const string TransactionId = "txn_dotnet";
var secret = Encoding.UTF8.GetBytes("0123456789abcdef0123456789abcdef");
var body = Encoding.UTF8.GetBytes("hello");
var headers = new Dictionary<string, string>
{
    ["x-agentpay-signature"] = Sign(secret, body, Timestamp, TransactionId),
    ["x-agentpay-timestamp"] = Timestamp,
    ["x-agentpay-transaction-id"] = TransactionId,
};
var verifier = new AgentPayVerifier(secret, new MemoryReplayStore());
await verifier.VerifyAsync("POST", "/fulfill", body, headers, DateTimeOffset.Parse(Timestamp));
await AssertRejectedAsync(
    verifier,
    body,
    headers,
    DateTimeOffset.Parse(Timestamp),
    "replay");

var modifiedVerifier = new AgentPayVerifier(secret, new MemoryReplayStore());
await AssertRejectedAsync(
    modifiedVerifier,
    Encoding.UTF8.GetBytes("changed"),
    headers,
    DateTimeOffset.Parse(Timestamp),
    "invalid_signature");

var staleHeaders = new Dictionary<string, string>(headers)
{
    ["x-agentpay-signature"] = Sign(secret, body, Timestamp, "txn_stale"),
    ["x-agentpay-transaction-id"] = "txn_stale",
};
var staleVerifier = new AgentPayVerifier(secret, new MemoryReplayStore());
await AssertRejectedAsync(
    staleVerifier,
    body,
    staleHeaders,
    DateTimeOffset.Parse(Timestamp).AddMinutes(6),
    "stale_request");

try
{
    _ = new AgentPayVerifier(Encoding.UTF8.GetBytes("short"), new MemoryReplayStore());
    throw new Exception("short secret was accepted");
}
catch (ArgumentException)
{
    // The constructor must reject weak signing secrets before any request is accepted.
}

var middlewareTimestamp = DateTimeOffset.UtcNow.ToString("O");
var middlewareTransactionId = "txn_dotnet_middleware";
var middlewareContext = new DefaultHttpContext();
middlewareContext.Request.Method = "POST";
middlewareContext.Request.Path = "/fulfill";
middlewareContext.Request.Body = new MemoryStream(body);
middlewareContext.Request.Headers["x-agentpay-signature"] = Sign(
    secret,
    body,
    middlewareTimestamp,
    middlewareTransactionId);
middlewareContext.Request.Headers["x-agentpay-timestamp"] = middlewareTimestamp;
middlewareContext.Request.Headers["x-agentpay-transaction-id"] = middlewareTransactionId;
string? capturedBody = null;
var middleware = new AgentPayVerificationMiddleware(
    async context =>
    {
        using var reader = new StreamReader(context.Request.Body, Encoding.UTF8, leaveOpen: true);
        capturedBody = await reader.ReadToEndAsync();
        context.Response.StatusCode = StatusCodes.Status204NoContent;
    },
    new AgentPayVerifier(secret, new MemoryReplayStore()));
await middleware.InvokeAsync(middlewareContext);
if (middlewareContext.Response.StatusCode != StatusCodes.Status204NoContent || capturedBody != "hello")
{
    throw new Exception("middleware did not preserve the verified request body");
}

// AssertRejectedAsync verifies one expected stable failure code.
static async Task AssertRejectedAsync(
    AgentPayVerifier verifier,
    byte[] body,
    IReadOnlyDictionary<string, string> headers,
    DateTimeOffset now,
    string expectedCode)
{
    try
    {
        await verifier.VerifyAsync("POST", "/fulfill", body, headers, now);
        throw new Exception($"{expectedCode} request was accepted");
    }
    catch (VerificationException exception) when (exception.ErrorCode == expectedCode)
    {
    }
}

/// <summary>Creates an independent known-answer request signature.</summary>
static string Sign(byte[] secret, byte[] body, string timestamp, string transactionId)
{
    var bodyHash = Convert.ToHexString(SHA256.HashData(body)).ToLowerInvariant();
    var canonical = string.Join("\n", "agentpay.seller-request.v1", timestamp, "POST", "/fulfill", bodyHash, transactionId);
    return Convert.ToBase64String(HMACSHA256.HashData(secret, Encoding.UTF8.GetBytes(canonical)));
}
