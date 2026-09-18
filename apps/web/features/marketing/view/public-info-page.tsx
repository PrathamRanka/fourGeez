import type { ReactNode } from "react";

type PublicInfoPageProps = {
  children: ReactNode;
  eyebrow: string;
  summary: string;
  title: string;
};

// PublicInfoPage provides a consistent editorial layout for supporting public routes.
export function PublicInfoPage({ children, eyebrow, summary, title }: PublicInfoPageProps) {
  return (
    <main id="main-content" className="min-h-[70svh]">
      <section className="hero-surface relative border-b border-border py-[clamp(5rem,12vw,9rem)]">
        <div className="site-container relative z-10">
          <p className="section-kicker">{eyebrow}</p>
          <h1 className="mt-5 max-w-[14ch] font-display text-[clamp(3rem,7vw,6.5rem)] font-semibold leading-[0.95] tracking-[-0.07em]">
            {title}
          </h1>
          <p className="section-copy mt-6 max-w-2xl">{summary}</p>
        </div>
      </section>
      <section className="site-container py-[clamp(3.5rem,8vw,7rem)]">
        <div className="public-copy max-w-3xl">{children}</div>
      </section>
    </main>
  );
}
