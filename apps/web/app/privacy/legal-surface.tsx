import styles from "./legal-surface.module.css";

export function LegalPage({
  indexLabel,
  eyebrow,
  title,
  summary,
  sections,
}: {
  indexLabel: string;
  eyebrow: string;
  title: string;
  summary: string;
  sections: readonly (readonly [string, string])[];
}) {
  return (
    <main id="main-content" className={styles.page}>
      <div className={styles.rail}>
        <aside className={styles.index}>
          <p className={styles.indexLabel}>{indexLabel}</p>
          <ol>
            {sections.map(([heading], index) => (
              <li key={heading}>
                <span>0{index + 1}</span>
                {heading}
              </li>
            ))}
          </ol>
        </aside>
        <article className={styles.content}>
          <p className={styles.eyebrow}>{eyebrow}</p>
          <h1>{title}</h1>
          <p className={styles.summary}>{summary}</p>
          <div className={styles.sections}>
            {sections.map(([heading, copy]) => (
              <section className={styles.section} key={heading}>
                <h2>{heading}</h2>
                <p>{copy}</p>
              </section>
            ))}
          </div>
        </article>
      </div>
    </main>
  );
}
