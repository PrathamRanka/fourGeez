import Link from "next/link";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { frequentlyAskedQuestions } from "@/features/marketing/model";

// FrequentlyAskedQuestions answers the most important launch, payment, and discovery concerns.
export function FrequentlyAskedQuestions() {
  return (
    <section id="faq" className="faq-section">
      <div className="site-container faq-layout">
        <div>
          <h2 className="faq-title">Questions sellers ask.</h2>
          <Link className="faq-doc-link" href="/docs">
            Browse the documentation →
          </Link>
        </div>
        <Accordion className="faq-accordion">
          {frequentlyAskedQuestions.map((question, index) => (
            <AccordionItem key={question.question} value={`question-${index}`}>
              <AccordionTrigger className="faq-trigger">
                {question.question}
              </AccordionTrigger>
              <AccordionContent className="faq-content">
                <p>{question.answer}</p>
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </div>
    </section>
  );
}
