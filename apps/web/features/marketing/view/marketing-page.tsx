import { CapabilityGrid } from "@/features/marketing/view/capability-grid";
import { ClosingCallToAction } from "@/features/marketing/view/closing-call-to-action";
import { CommerceNetwork } from "@/features/marketing/view/commerce-network";
import { FrequentlyAskedQuestions } from "@/features/marketing/view/frequently-asked-questions";
import { HeroSection } from "@/features/marketing/view/hero-section";
import { LaunchWorkflow } from "@/features/marketing/view/launch-workflow";
import { PricingPreview } from "@/features/marketing/view/pricing-preview";
import { TrustAndPayments } from "@/features/marketing/view/trust-and-payments";

// MarketingPage composes the public AgentPay story in reading order.
export function MarketingPage() {
  return (
    <main id="main-content" className="overflow-hidden">
      <HeroSection />
      <LaunchWorkflow />
      <CapabilityGrid />
      <CommerceNetwork />
      <TrustAndPayments />
      <PricingPreview />
      <FrequentlyAskedQuestions />
      <ClosingCallToAction />
    </main>
  );
}
