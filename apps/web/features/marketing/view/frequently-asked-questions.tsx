import Link from "next/link";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { frequentlyAskedQuestions } from "@/features/marketing/model";
import styles from "./marketing-page.module.css";

export function FrequentlyAskedQuestions() {
  const structuredData = {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: frequentlyAskedQuestions.map(({ question, answer }) => ({
      "@type": "Question",
      name: question,
      acceptedAnswer: {
        "@type": "Answer",
        text: answer,
      },
    })),
  };

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify(structuredData).replace(/</g, "\\u003c"),
        }}
      />
      <section
        id="faq"
        className={styles.faqSection}
        aria-labelledby="faq-title"
      >
        <div className={styles.faqIntro}>
          <p className={styles.kicker}>FAQ</p>
          <h2 id="faq-title">Clear before you connect.</h2>
          <p>Payments, access, discovery, and control—without protocol fog.</p>
          <Link href="/docs">Browse documentation →</Link>
        </div>
        <Accordion className={styles.faqList}>
          {frequentlyAskedQuestions.map((question, index) => (
            <AccordionItem
              className={styles.faqItem}
              key={question.question}
              value={`question-${index}`}
            >
              <AccordionTrigger className={styles.faqTrigger}>
                {question.question}
              </AccordionTrigger>
              <AccordionContent className={styles.faqContent}>
                <p>{question.answer}</p>
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </section>
    </>
  );
}
