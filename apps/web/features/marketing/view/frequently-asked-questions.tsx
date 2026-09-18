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
    <section id="faq" className="border-t border-border bg-card py-[var(--section-space)]">
      <div className="site-container grid gap-10 lg:grid-cols-[0.7fr_1.3fr] lg:gap-20">
        <div>
          <p className="section-kicker">Clear before you connect</p>
          <h2 className="section-title mt-4">Questions, answered.</h2>
        </div>
        <Accordion className="border-t border-border">
          {frequentlyAskedQuestions.map((question, index) => (
            <AccordionItem key={question.question} value={`question-${index}`}>
              <AccordionTrigger className="py-6 font-display text-lg font-semibold tracking-[-0.025em] hover:no-underline">
                {question.question}
              </AccordionTrigger>
              <AccordionContent className="max-w-2xl pb-6 text-base leading-7 text-muted-foreground">
                <p>{question.answer}</p>
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </div>
    </section>
  );
}
