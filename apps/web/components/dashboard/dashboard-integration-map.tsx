import { Bot, Cloud, Code2 } from "lucide-react";
import styles from "./dashboard-visuals.module.css";

export function DashboardIntegrationMap({ connected = false }: { connected?: boolean }) {
  return (
    <section className={styles.integration} aria-label="AgentPay MCP connection">
      <svg className={styles.integrationLines} viewBox="0 0 640 260" preserveAspectRatio="none" aria-hidden="true">
        <path d="M106 52 C220 52 202 130 320 130 S430 208 534 208" fill="none" stroke="currentColor" />
        <path d="M106 52 C220 52 202 130 320 130 S430 208 534 208" fill="none" stroke="url(#agentpay-flow)" strokeWidth="2" />
        <defs>
          <linearGradient id="agentpay-flow" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0" stopColor="transparent" />
            <stop offset="0.5" stopColor="var(--dash-blue, #4b73e8)" />
            <stop offset="1" stopColor="var(--dash-pink, #d99cc0)" />
          </linearGradient>
        </defs>
      </svg>
      <div className={styles.node} data-node="repository">
        <Code2 aria-hidden="true" />
        <div><strong>Seller repository</strong><span>{connected ? "Connector active" : "Add connector"}</span></div>
      </div>
      <div className={styles.node} data-node="cloud">
        <Cloud aria-hidden="true" />
        <div><strong>AgentPay cloud</strong><span>Authorizes commerce</span></div>
      </div>
      <div className={styles.node} data-node="agents">
        <Bot aria-hidden="true" />
        <div><strong>Buyer agents</strong><span>Discover and pay</span></div>
      </div>
    </section>
  );
}
