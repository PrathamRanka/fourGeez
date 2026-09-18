import type { Metadata } from "next";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = {
  title: "Documentation",
  description: "The AgentPay seller integration path for coding agents and existing APIs.",
};

const integrationSteps = [
  ["Create a project", "Create a seller storefront and issue one project-scoped integration key."],
  ["Connect your agent", "Add the AgentPay MCP server to Claude Code, Codex, or a generic MCP host."],
  ["Generate the integration", "Ask the coding agent to propose routes, prices, verification, discovery files, and tests."],
  ["Review and verify", "Approve the proposed changes, prove wallet control, and run the sandbox purchase."],
  ["Publish", "Publish only after AgentPay verifies signatures, payment gating, metadata, and fulfillment."],
] as const;

// DocsPage introduces the supported seller integration sequence.
export default function DocsPage() {
  return (
    <PublicInfoPage
      eyebrow="Documentation"
      title="Connect AgentPay to your repository."
      summary="The V1 path is deliberately small: one scoped key, one MCP connection, one reviewable integration proposal."
    >
      <ol className="grid gap-4">
        {integrationSteps.map(([title, description], index) => (
          <li key={title} className="grid gap-4 rounded-2xl border border-border bg-card p-5 sm:grid-cols-[3rem_1fr]">
            <span className="font-mono text-xs text-primary">0{index + 1}</span>
            <div>
              <h2>{title}</h2>
              <p>{description}</p>
            </div>
          </li>
        ))}
      </ol>
      <div className="mt-10 rounded-2xl border border-primary/15 bg-primary/5 p-5">
        <h2>Representative setup prompt</h2>
        <p className="font-mono text-sm">
          Connect this project to AgentPay. Identify sellable API routes, propose products and
          prices, install request verification, generate the storefront and discovery metadata,
          run the tests, and prepare the changes for my approval.
        </p>
      </div>
    </PublicInfoPage>
  );
}
