import type { ReactNode } from "react";
import Link from "next/link";
import { BrandMark } from "@/components/site/brand-mark";
import styles from "./legal-surface.module.css";

export const LEGAL_EFFECTIVE_DATE = "September 19, 2026";

export type LegalSection = {
  id: string;
  title: string;
  paragraphs: readonly ReactNode[];
  bullets?: readonly ReactNode[];
  note?: ReactNode;
};

const legalLinks = [
  { href: "/privacy", label: "Privacy" },
  { href: "/terms", label: "Terms" },
  { href: "/security", label: "Security" },
] as const;

export function LegalPage({
  currentPath,
  indexLabel,
  eyebrow,
  title,
  summary,
  notice,
  sections,
}: {
  currentPath: (typeof legalLinks)[number]["href"];
  indexLabel: string;
  eyebrow: string;
  title: string;
  summary: string;
  notice: ReactNode;
  sections: readonly LegalSection[];
}) {
  return (
    <main id="main-content" className={styles.page}>
      <header className={styles.header}>
        <Link href="/" className={styles.brand} aria-label="AgentPay home">
          <BrandMark />
        </Link>
        <nav className={styles.legalNav} aria-label="Legal pages">
          {legalLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              aria-current={currentPath === link.href ? "page" : undefined}
            >
              {link.label}
            </Link>
          ))}
        </nav>
      </header>

      <div className={styles.accentRule} aria-hidden="true" />

      <div className={styles.hero}>
        <div className={styles.heroCopy}>
          <p className={styles.eyebrow}>{eyebrow}</p>
          <h1>{title}</h1>
          <p className={styles.summary}>{summary}</p>
        </div>
        <dl className={styles.documentFacts}>
          <div>
            <dt>Status</dt>
            <dd>Pre-launch</dd>
          </div>
          <div>
            <dt>Effective</dt>
            <dd>
              <time dateTime="2026-09-19">{LEGAL_EFFECTIVE_DATE}</time>
            </dd>
          </div>
          <div>
            <dt>Review</dt>
            <dd>External review pending</dd>
          </div>
        </dl>
      </div>

      <section className={styles.notice} aria-labelledby="prelaunch-heading">
        <span className={styles.noticeMarker} aria-hidden="true" />
        <div>
          <h2 id="prelaunch-heading">Pre-launch information</h2>
          <p>{notice}</p>
        </div>
      </section>

      <div className={styles.rail}>
        <aside className={styles.index}>
          <nav aria-label={`${eyebrow} sections`}>
            <p className={styles.indexLabel}>{indexLabel}</p>
            <ol>
              {sections.map((section, index) => (
                <li key={section.id}>
                  <Link href={`#${section.id}`}>
                    <span>{String(index + 1).padStart(2, "0")}</span>
                    {section.title}
                  </Link>
                </li>
              ))}
            </ol>
          </nav>
          <Link href="/docs" className={styles.docsLink}>
            Read the technical documentation
            <span aria-hidden="true">↗</span>
          </Link>
        </aside>

        <article className={styles.content}>
          <div className={styles.sections}>
            {sections.map((section, index) => (
              <section
                className={styles.section}
                id={section.id}
                key={section.id}
                aria-labelledby={`${section.id}-heading`}
              >
                <div className={styles.sectionNumber} aria-hidden="true">
                  {String(index + 1).padStart(2, "0")}
                </div>
                <div className={styles.sectionBody}>
                  <h2 id={`${section.id}-heading`}>{section.title}</h2>
                  {section.paragraphs.map((paragraph, paragraphIndex) => (
                    <p key={`${section.id}-paragraph-${paragraphIndex}`}>
                      {paragraph}
                    </p>
                  ))}
                  {section.bullets ? (
                    <ul>
                      {section.bullets.map((bullet, bulletIndex) => (
                        <li key={`${section.id}-bullet-${bulletIndex}`}>
                          {bullet}
                        </li>
                      ))}
                    </ul>
                  ) : null}
                  {section.note ? (
                    <aside className={styles.sectionNote}>{section.note}</aside>
                  ) : null}
                </div>
              </section>
            ))}
          </div>

          <footer className={styles.documentFooter}>
            <p>
              Effective and last updated:{" "}
              <time dateTime="2026-09-19">{LEGAL_EFFECTIVE_DATE}</time>
            </p>
            <Link href="/">Return to AgentPay</Link>
          </footer>
        </article>
      </div>
    </main>
  );
}
