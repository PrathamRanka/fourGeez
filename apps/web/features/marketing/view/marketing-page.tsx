import { ClosingCallToAction } from "@/features/marketing/view/closing-call-to-action";
import { FrequentlyAskedQuestions } from "@/features/marketing/view/frequently-asked-questions";
import { HeroSection } from "@/features/marketing/view/hero-section";
import { ManifestoSection } from "@/features/marketing/view/manifesto-section";
import { PartnerStrip } from "@/features/marketing/view/partner-strip";
import { ProductShowcase } from "@/features/marketing/view/product-showcase";

// MarketingPage composes the public AgentPay story in reading order.
export function MarketingPage() {
  return (
    <main id="main-content" className="overflow-hidden">
      <HeroSection />
      <PartnerStrip />
      <ManifestoSection />
      <ProductShowcase />
      <FrequentlyAskedQuestions />
      <ClosingCallToAction />
    </main>
  );
}
