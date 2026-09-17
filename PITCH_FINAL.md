# AgentPay — Final Pitch Outline

## Hook (15 sec)
"AI agents are about to start paying for things on their own. Every rail that lets them do it has shipped the ability to pay before the ability to prove, audit, or undo it — and none of them handle the purchases that need more than one person's approval."

## The gap, with receipts (30 sec)
- x402's own documentation: irreversible settlement, no dispute mechanism.
- Legally unresolved whether an agent's authorization satisfies Regulation E.
- Checked ZeroClick.ai (live seller-agent-commerce product) — same gap, publicly visible.
- Precedent: Chargehound and Verifi did this for human payments, got acquired by PayPal and Visa.
- Separately: nobody's built agent commerce for decisions that aren't one person's call — families, co-founders, procurement teams.

## Live demo (90 sec)
Full script from HACKATHON_PLAN_FINAL.md: agent proposes a purchase → multi-party live approval → evidence bundle → dispute → auto-resolution.

## Why this is defensible (30 sec)
- Same drop-in proxy shape as an existing live product (ZeroClick), plus the trust layer they don't have.
- Cross-seller agent trust network — reputation data no single-seller-scoped competitor can replicate.
- Multi-party authorization is a structurally hard problem to bolt on later — it's built into the evidence schema from day one, not an afterthought.
- RL improves negotiation without touching the safety-critical path — a stronger technical story than "we use AI everywhere."

## Where it goes (15 sec)
Dispute API today → cross-seller risk scoring → multi-party commerce infrastructure for every kind of joint spending decision → the Signifyd/Forter of agentic commerce.

## Team & stack note
Next.js/TypeScript frontend, Go backend (the security boundary between reasoning and money movement), Bedrock for reasoning, fully on AWS — a team that can already ship this, not just describe it.

## Ask / close
Hackathon: what's demoed today, what's next.
YC: traction (design partners, transaction volume once real), team, specific ask.

---

## Honest risks to have answers ready for
- Is this three ideas in a trenchcoat? Answer: no — evidence/dispute is the core; RL and multi-party are both extensions of the *same* evidence and policy engine, not separate systems bolted together. Be ready to explain this clearly if asked, because it's the most likely pushback.
- Platform risk: could Locus, Blaze, or ZeroClick add dispute handling themselves? Answer honestly: yes, that's the biggest risk, and speed + the cross-seller trust network are the mitigations.
- Confirm before pitching: what ZeroClick's actual current dispute/reputation handling is (don't assume from public docs alone), and get at least one real conversation with a seller live on x402/ACP about actual current pain.
