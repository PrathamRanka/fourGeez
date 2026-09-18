import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";

// ClosingCallToAction gives sellers one clear next step after reviewing the product.
export function ClosingCallToAction() {
  return (
    <section className="closing-surface">
      <div className="closing-orb" aria-hidden="true" />
      <div className="site-container closing-content">
        <h2>
          Turn agent traffic into
          <br />
          paying customers.
        </h2>
        <div className="closing-actions">
          <Link
            href="/sign-up"
            className={buttonVariants({ size: "lg", className: "px-4" })}
          >
            Start selling with AgentPay
            <ArrowRight aria-hidden="true" />
          </Link>
          <Link
            href="/sign-in"
            className={buttonVariants({ variant: "outline", size: "lg" })}
          >
            Sign in
          </Link>
        </div>
      </div>
    </section>
  );
}
