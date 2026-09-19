import { ImageResponse } from "next/og";
import { AGENTPAY_SYMBOL_PATH } from "@/components/site/brand-symbol";

export const alt = "AgentPay — seller-first x402 API storefronts for AI agents";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpenGraphImage() {
  return new ImageResponse(
    <div
      style={{
        alignItems: "center",
        background: "#050506",
        color: "#f5f5f7",
        display: "flex",
        height: "100%",
        justifyContent: "space-between",
        padding: "78px 88px",
        width: "100%",
      }}
    >
      <div style={{ display: "flex", flexDirection: "column", width: 780 }}>
        <div
          style={{
            color: "#a1a1aa",
            display: "flex",
            fontSize: 22,
            letterSpacing: "0.14em",
            textTransform: "uppercase",
          }}
        >
          Seller-first commerce infrastructure
        </div>
        <div
          style={{
            display: "flex",
            fontSize: 72,
            fontWeight: 700,
            letterSpacing: "-0.055em",
            lineHeight: 1.02,
            marginTop: 26,
          }}
        >
          Turn an API into an agent-ready storefront.
        </div>
        <div
          style={{
            color: "#b8b8bf",
            display: "flex",
            fontSize: 27,
            lineHeight: 1.45,
            marginTop: 28,
          }}
        >
          MCP integration · x402 testnet checkout · signed fulfillment
        </div>
      </div>

      <div
        style={{
          alignItems: "center",
          border: "2px solid #27272a",
          borderRadius: 48,
          display: "flex",
          height: 250,
          justifyContent: "center",
          position: "relative",
          width: 250,
        }}
      >
        <div
          style={{
            background: "linear-gradient(90deg, #2979ff, #ff5aa5)",
            height: 8,
            left: 28,
            position: "absolute",
            right: 28,
            top: 26,
          }}
        />
        <svg viewBox="0 0 48 48" width="150" height="150">
          <path d={AGENTPAY_SYMBOL_PATH} fill="#f5f5f7" fillRule="evenodd" />
        </svg>
      </div>
    </div>,
    size,
  );
}
