import { beforeEach, describe, expect, it, vi } from "vitest";
import { GET, POST } from "@/app/api/backend/[...path]/route";

describe("branded backend proxy", () => {
  beforeEach(() => {
    vi.stubEnv(
      "AGENTPAY_API_ORIGIN",
      "https://sadmp7j94e.execute-api.ap-south-1.amazonaws.com",
    );
    vi.stubEnv("AGENTPAY_WEB_ORIGIN", "https://agentpay.prathamranka.in");
  });

  it("forwards public API reads to the fixed server-only origin", async () => {
    const upstreamFetch = vi.fn(async () =>
      Response.json(
        { apiOrigin: "https://agentpay.prathamranka.in/api/backend" },
        {
          headers: {
            "Cache-Control": "public, max-age=60",
            "X-AgentPay-Request-Id": "req_public",
          },
        },
      ),
    );
    vi.stubGlobal("fetch", upstreamFetch);

    const response = await GET(
      new Request(
        "https://agentpay.prathamranka.in/api/backend/.well-known/agentpay?source=test",
      ),
      { params: Promise.resolve({ path: [".well-known", "agentpay"] }) },
    );

    expect(upstreamFetch).toHaveBeenCalledWith(
      "https://sadmp7j94e.execute-api.ap-south-1.amazonaws.com/.well-known/agentpay?source=test",
      expect.objectContaining({ method: "GET", cache: "no-store" }),
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("x-agentpay-request-id")).toBe("req_public");
  });

  it("forwards only allowlisted request headers and preserves payment responses", async () => {
    const upstreamFetch = vi.fn(async (_url: string, init?: RequestInit) => {
      expect(new Headers(init?.headers).get("authorization")).toBe("Bearer buyer");
      expect(new Headers(init?.headers).get("cookie")).toBeNull();
      expect(init?.body).toBeInstanceOf(ArrayBuffer);
      return Response.json(
        { error: { code: "payment_required" } },
        {
          status: 402,
          headers: {
            "Payment-Required": "challenge",
            "X-AgentPay-Request-Id": "req_payment",
          },
        },
      );
    });
    vi.stubGlobal("fetch", upstreamFetch);

    const response = await POST(
      new Request(
        "https://agentpay.prathamranka.in/api/backend/pay/store/report",
        {
          method: "POST",
          headers: {
            Authorization: "Bearer buyer",
            Cookie: "private-session=do-not-forward",
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ audience: "founders" }),
        },
      ),
      { params: Promise.resolve({ path: ["pay", "store", "report"] }) },
    );

    expect(response.status).toBe(402);
    expect(response.headers.get("payment-required")).toBe("challenge");
    expect(response.headers.get("x-agentpay-request-id")).toBe("req_payment");
  });

  it("fails closed when the private origin points back to the branded proxy", async () => {
    vi.stubEnv(
      "AGENTPAY_API_ORIGIN",
      "https://agentpay.prathamranka.in/api/backend",
    );
    const upstreamFetch = vi.fn();
    vi.stubGlobal("fetch", upstreamFetch);

    const response = await GET(
      new Request(
        "https://agentpay.prathamranka.in/api/backend/.well-known/agentpay",
      ),
      { params: Promise.resolve({ path: [".well-known", "agentpay"] }) },
    );

    expect(response.status).toBe(503);
    expect(upstreamFetch).not.toHaveBeenCalled();
  });
});
