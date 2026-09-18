import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { loadApprovalSession } from "@/features/approvals/controller";
import { ApprovalSession } from "@/features/approvals/view/approval-session";

export const metadata: Metadata = {
  title: "Purchase approval",
  robots: { index: false, follow: false },
};

type ApprovalPageProps = {
  params: Promise<{ sessionId: string }>;
  searchParams: Promise<{ token?: string }>;
};

export default async function ApprovalPage({ params, searchParams }: ApprovalPageProps) {
  const [{ sessionId }, query] = await Promise.all([params, searchParams]);
  if (!query.token) {
    notFound();
  }
  const result = await loadApprovalSession(sessionId, query.token);
  if (!result.ok) {
    notFound();
  }
  return (
    <ApprovalSession
      initialSnapshot={result.value}
      now={new Date().toISOString()}
      websocketOrigin={process.env.AGENTPAY_WS_ORIGIN}
    />
  );
}

