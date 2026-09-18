import { ArrowUpRight, Check, LockKeyhole, Sparkles } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import styles from "./auth-surface.module.css";

type AuthSurfaceProps = {
  children: ReactNode;
  eyebrow: string;
  highlight: string;
  summary: string;
  title: string;
};

const trustPoints = [
  "Same-origin sessions",
  "Scoped seller access",
  "No wallet keys stored",
] as const;

export function AuthSurface({
  children,
  eyebrow,
  highlight,
  summary,
  title,
}: AuthSurfaceProps) {
  return (
    <main id="main-content" className={styles.page}>
      <div className={styles.rail}>
        <section className={styles.story} aria-label="AgentPay seller access">
          <div className={styles.storyGlow} aria-hidden="true" />
          <div className={styles.storyTopline}>
            <span>
              <Sparkles aria-hidden="true" /> Seller control plane
            </span>
            <span>01 / Access</span>
          </div>
          <div className={styles.storyCopy}>
            <p className={styles.eyebrow}>{eyebrow}</p>
            <h1>{title}</h1>
            <p>{summary}</p>
          </div>
          <div className={styles.preview}>
            <div className={styles.previewHeader}>
              <span>{highlight}</span>
              <LockKeyhole aria-hidden="true" />
            </div>
            <div className={styles.previewBody}>
              <div className={styles.signal} aria-hidden="true">
                <span />
                <span />
                <span />
              </div>
              <strong>Seller identity verified before commerce access.</strong>
              <ul>
                {trustPoints.map((point) => (
                  <li key={point}>
                    <Check aria-hidden="true" /> {point}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </section>
        <section className={styles.formColumn}>
          <div className={styles.formTopline}>
            <Link href="/">AgentPay</Link>
            <Link href="/docs">
              Setup guide <ArrowUpRight aria-hidden="true" />
            </Link>
          </div>
          <div className={styles.formStage}>{children}</div>
          <p className={styles.formFootnote}>
            AgentPay verifies identity, entitlement, and seller authority at
            every protected boundary.
          </p>
        </section>
      </div>
    </main>
  );
}
