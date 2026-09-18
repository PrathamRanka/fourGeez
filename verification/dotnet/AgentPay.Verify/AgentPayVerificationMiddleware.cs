using Microsoft.AspNetCore.Http;

namespace AgentPay.Verify;

/// <summary>Verifies ASP.NET Core requests before seller fulfillment.</summary>
public sealed class AgentPayVerificationMiddleware(RequestDelegate next, AgentPayVerifier verifier)
{
    private const int MaximumBodyBytes = 1024 * 1024;

    /// <summary>Reads and restores the request body before calling the next middleware.</summary>
    public async Task InvokeAsync(HttpContext context)
    {
        context.Request.EnableBuffering();
        using var bodyStream = new MemoryStream();
        await context.Request.Body.CopyToAsync(bodyStream, context.RequestAborted);
        if (bodyStream.Length > MaximumBodyBytes)
        {
            await RejectAsync(context, StatusCodes.Status401Unauthorized);
            return;
        }
        context.Request.Body.Position = 0;
        var headers = context.Request.Headers.ToDictionary(
            header => header.Key,
            header => header.Value.ToString(),
            StringComparer.OrdinalIgnoreCase);
        try
        {
            await verifier.VerifyAsync(
                context.Request.Method,
                context.Request.Path.Value ?? "/",
                bodyStream.ToArray(),
                headers,
                DateTimeOffset.UtcNow,
                context.RequestAborted);
            await next(context);
        }
        catch (VerificationException exception)
        {
            await RejectAsync(
                context,
                exception.ErrorCode == "replay"
                    ? StatusCodes.Status409Conflict
                    : StatusCodes.Status401Unauthorized);
        }
        catch
        {
            await RejectAsync(context, StatusCodes.Status503ServiceUnavailable);
        }
    }

    /// <summary>Returns the shared redacted middleware error response.</summary>
    private static async Task RejectAsync(HttpContext context, int statusCode)
    {
        context.Response.StatusCode = statusCode;
        context.Response.ContentType = "application/json";
        await context.Response.WriteAsync("{\"error\":\"AgentPay verification failed\"}");
    }
}
