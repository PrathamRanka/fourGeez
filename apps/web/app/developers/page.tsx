import {
  ArrowLeft,
  ArrowUpRight,
  BriefcaseBusiness,
  Code2,
} from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { developersMetadata } from "@/features/marketing/seo";
import styles from "./developers.module.css";

export const metadata: Metadata = developersMetadata;

const developers = [
  {
    name: "Pratham Ranka",
    role: "Product and engineering",
    summary:
      "Building the seller experience, product direction, and the systems that turn APIs into agent-ready storefronts.",
    github: "https://github.com/PrathamRanka",
    linkedin: "https://www.linkedin.com/in/prathamranka06/",
    image: "/developers/pratham-ranka.png",
  },
  {
    name: "Ayush Garg",
    role: "Engineering",
    summary:
      "Building and maintaining AgentPay's application foundations, integrations, and production delivery path.",
    github: "https://github.com/gargayush1911",
    linkedin: "https://www.linkedin.com/in/gargayush1911/",
    image: "/developers/ayush-garg.png",
  },
] as const;

export default function DevelopersPage() {
  return (
    <main id="main-content" className={styles.page}>
      <section className={styles.intro}>
        <div className={`site-container ${styles.introInner}`}>
          <Link className={styles.backLink} href="/">
            <ArrowLeft aria-hidden="true" />
            Back to AgentPay
          </Link>
          <p className={styles.kicker}>AgentPay / Developers</p>
          <h1>Built by people who ship.</h1>
          <p className={styles.lede}>
            A small team working across product, infrastructure, and developer
            experience to make agent commerce practical for API sellers.
          </p>
        </div>
      </section>

      <section
        className={`site-container ${styles.team}`}
        aria-label="AgentPay developers"
      >
        {developers.map((developer, index) => (
          <article className={styles.card} key={developer.name}>
            <div className={styles.portrait}>
              <div
                role="img"
                aria-label={`${developer.name}, AgentPay developer`}
                className={styles.image}
                style={{ backgroundImage: `url(${developer.image})` }}
              />
              <span className={styles.index}>0{index + 1}</span>
            </div>

            <div className={styles.details}>
              <p className={styles.role}>{developer.role}</p>
              <h2>{developer.name}</h2>
              <p className={styles.summary}>{developer.summary}</p>
              <div className={styles.links}>
                <a
                  href={developer.github}
                  target="_blank"
                  rel="noreferrer"
                  aria-label={`${developer.name} on GitHub`}
                >
                  <Code2 aria-hidden="true" />
                  GitHub
                  <ArrowUpRight aria-hidden="true" />
                </a>
                <a
                  href={developer.linkedin}
                  target="_blank"
                  rel="noreferrer"
                  aria-label={`${developer.name} on LinkedIn`}
                >
                  <BriefcaseBusiness aria-hidden="true" />
                  LinkedIn
                  <ArrowUpRight aria-hidden="true" />
                </a>
              </div>
            </div>
          </article>
        ))}
      </section>
    </main>
  );
}
