import { ArrowUpRight, Check } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { BrandMark } from "@/components/site/brand-mark";
import styles from "./auth-surface.module.css";

type AuthSurfaceProps = {
  children: ReactNode;
  eyebrow: string;
  highlight: string;
  summary: string;
  title: string;
};

const trustPoints = [
  "Create your storefront",
  "Connect your payment wallet",
  "Publish products for agents",
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
          <div className={styles.storyTopline}>
            <Link href="/" aria-label="AgentPay home">
              <BrandMark />
            </Link>
            <span>Seller workspace</span>
          </div>
          <div className={styles.storyCopy}>
            <p className={styles.eyebrow}>{eyebrow}</p>
            <h1>{title}</h1>
            <p>{summary}</p>
          </div>
          <div className={styles.trustBlock}>
            <span>{highlight}</span>
            <ul>
              {trustPoints.map((point) => (
                <li key={point}>
                  <Check aria-hidden="true" /> {point}
                </li>
              ))}
            </ul>
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
          <p className={styles.formFootnote}>Secure seller access by AgentPay.</p>
        </section>
      </div>
    </main>
  );
}
