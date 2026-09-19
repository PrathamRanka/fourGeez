# Contributing to AgentPay

AgentPay is a proprietary-source project. It is not an open-source project and
the maintainers are not obligated to accept, merge, publish, or compensate any
submission.

## Before contributing

- Open an issue for non-trivial proposals unless an owner requested the work.
- Never submit secrets, private customer data, wallet material, access tokens,
  third-party confidential information, or code you are not authorized to use.
- Keep one documented implementation task and one logical change per pull
  request.
- Follow `AGENTS.md`, the controlling product/API/data contracts, and the test
  requirements in this repository.
- Do not introduce code with a license that conflicts with proprietary
  distribution. Identify every copied component and its license in the pull
  request.

## Contribution rights

You retain ownership of copyright in your original contribution unless you and
the applicable repository owners sign a separate written assignment.

By submitting a contribution, you grant Pratham Ranka and Ayush Garg a
perpetual, worldwide, non-exclusive, irrevocable, royalty-free, transferable,
and sublicensable license to use, reproduce, modify, prepare derivative works,
distribute, publicly display, publicly perform, commercialize, and relicense
the contribution as part of AgentPay or related products. You also grant any
patent license reasonably necessary to exercise those rights for patent claims
you can license that are necessarily infringed by your contribution.

This grant does not transfer ownership by itself. If exclusive ownership or an
assignment is required, contribution acceptance must wait for a separate
written contributor agreement reviewed by qualified counsel.

## Developer Certificate of Origin

Every commit must include a `Signed-off-by:` trailer certifying the Developer
Certificate of Origin 1.1. Add it with:

```text
git commit -s -m "type(scope): summary"
```

By signing off, you certify that you created the contribution or have the right
to submit it under these terms, and that the contribution may be maintained in
a public repository. A sign-off is not a copyright assignment.

## Pull-request requirements

1. Use a branch and pull request; do not push directly to `main`.
2. Explain the user-visible and security impact.
3. Include tests first for changed behavior and run the affected checks.
4. Update controlling documentation when a contract changes.
5. Preserve third-party notices and identify new dependency licenses.
6. Obtain the reviews and status checks required by repository rules.

The owners may close or reject a contribution for product, security, legal,
maintenance, or licensing reasons.
