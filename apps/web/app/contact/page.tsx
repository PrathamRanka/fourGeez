import { ArrowLeft, ArrowUpRight, Mail, Phone } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { contactMetadata } from "@/features/marketing/seo";
import styles from "./contact.module.css";

export const metadata: Metadata = contactMetadata;

const contacts = [
  {
    name: "Pratham Ranka",
    role: "Co-Founder",
    email: "pranka0789@gmail.com",
    phone: "+91 70232 06003",
    phoneHref: "tel:+917023206003",
  },
  {
    name: "Ayush Garg",
    role: "Co-Founder",
    email: "gargayush1911@gmail.com",
    phone: "+91 95887 91911",
    phoneHref: "tel:+919588791911",
  },
] as const;

export default function ContactPage() {
  return (
    <main id="main-content" className={styles.page}>
      <section className={`site-container ${styles.hero}`}>
        <Link className={styles.backLink} href="/">
          <ArrowLeft aria-hidden="true" />
          Back to AgentPay
        </Link>
        <p className={styles.kicker}>AgentPay / Contact</p>
        <h1>Talk directly to the founders.</h1>
        <p className={styles.lede}>
          Questions about onboarding, your API stack, or launch access? Call or
          email us. You will reach the people building the product.
        </p>
      </section>

      <section
        className={`site-container ${styles.contactGrid}`}
        aria-label="Founder contact details"
      >
        {contacts.map((contact, index) => (
          <article className={styles.contactCard} key={contact.email}>
            <div className={styles.cardTopline}>
              <span>0{index + 1}</span>
              <span>{contact.role}</span>
            </div>
            <h2>{contact.name}</h2>
            <div className={styles.actions}>
              <a
                href={`mailto:${contact.email}`}
                aria-label={`Email ${contact.name}`}
              >
                <Mail aria-hidden="true" />
                <span>
                  <small>Mail us</small>
                  <strong>{contact.email}</strong>
                </span>
                <ArrowUpRight aria-hidden="true" />
              </a>
              <a href={contact.phoneHref} aria-label={`Call ${contact.name}`}>
                <Phone aria-hidden="true" />
                <span>
                  <small>Call us</small>
                  <strong>{contact.phone}</strong>
                </span>
                <ArrowUpRight aria-hidden="true" />
              </a>
            </div>
          </article>
        ))}
      </section>
    </main>
  );
}
