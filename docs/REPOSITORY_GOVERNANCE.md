# Repository ownership and GitHub governance

Last verified: 2026-09-19.

This document records repository controls, not legal advice. Qualified counsel
must approve ownership assignments, commercial licenses, contributor terms,
and enforcement strategy before production commercialization.

## Current repository posture

The canonical repository is `PrathamRanka/fourGeez`. On 2026-09-19 GitHub's
public repository API reported that it was public, owned by the personal
account `PrathamRanka`, allowed GitHub forking, had one fork, and had Issues,
Projects, and Wiki enabled. GitHub publicly evidenced both `@PrathamRanka` and
`@gargayush1911` as contributors. The repository had no detected license before
the proprietary notice in this change.

The repository is proprietary source. Public GitHub access and an all-rights-
reserved notice cannot make a public repository technically uncloneable or
unforkable. GitHub's Terms permit users to view public content and reproduce it
through GitHub's fork functionality. A proprietary license limits additional
legal permissions; it does not erase copies or disable GitHub controls.

If technical no-fork/no-clone control is required, make the repository private
and remove access for everyone who does not need it. Existing forks, downloads,
and local clones may continue to exist. For stronger shared ownership and
private-fork controls, transfer the repository to a GitHub organization owned
by Pratham Ranka and Ayush Garg, require two-factor authentication, make the
repository private, and disable private/internal repository forking at both the
organization and repository level where the plan exposes those settings.

## Required owner settings

These controls cannot be enforced by committed files alone. An administrator
must apply and verify them in GitHub.

### Access and repository features

1. Decide whether source visibility is intentional. If not, select **Settings ?
   General ? Danger Zone ? Change repository visibility ? Private**.
2. Prefer an organization with both owners instead of relying on one personal
   account. Give maintainers the least repository role they need and review
   collaborators and deploy keys quarterly.
3. Confirm `@gargayush1911` has explicit write access; otherwise CODEOWNERS
   cannot request or enforce that owner's review.
4. Disable Wiki and Projects unless they are actively maintained. Keep Issues
   only if the team will triage the committed templates.
5. Enable **Require contributors to sign off on web-based commits**.

### Main-branch ruleset

Create an active branch ruleset targeting the default branch `main`:

- block branch deletion and force pushes;
- require changes through pull requests;
- require at least one approval and a code-owner approval;
- dismiss stale approvals and require approval after the latest reviewable push;
- require all conversations to be resolved;
- require the branch to be current before merging;
- require these CI jobs: `contracts`, `backend`, `rl-scaffold`,
  `extended-verification`, `web`, and `terraform`;
- restrict direct pushes and keep bypass access limited to a documented
  emergency owner path; and
- require linear history.

Require signed commits only after both owners and every automation identity are
configured to sign; enabling it prematurely can block deployments and automated
maintenance. Protect release tags with a separate tag ruleset.

CODEOWNERS requests review but does not itself prevent a merge. The ruleset must
explicitly require code-owner approval.

### Security and automation

1. Enable private vulnerability reporting and verify that the Security tab
   displays **Report a vulnerability** while signed out or using a non-owner
   account.
2. Enable dependency graph, Dependabot alerts, Dependabot security updates,
   secret scanning, and push protection where the account plan supports them.
3. In **Actions ? General**, allow only GitHub-authored and explicitly approved
   actions, set workflow permissions to read-only by default, prevent Actions
   from creating or approving pull requests, and require approval for workflows
   from first-time external contributors.
4. Protect production and AWS environments with required reviewers and scoped
   OIDC deployment identities. Do not store long-lived AWS or Vercel keys in
   repository secrets.
5. Enable tag/release protection and preserve audit/security logs available to
   the account plan.

### Analytics and cookie governance

The website currently loads Vercel Web Analytics on every route through the
exact dependency pin `@vercel/analytics` `2.0.1`. No custom analytics events are
configured. The public privacy notice identifies the default page-view data and
the strictly necessary seller-session, CSRF, and browser-purchase cookies.

Before changing analytics configuration or adding another tracker:

1. update the public privacy notice and subprocessor inventory;
2. prevent secrets, personal identifiers, payment data, and internal IDs from
   appearing in URLs, query parameters, or custom events;
3. review route redaction and retention settings in Vercel;
4. determine with qualified counsel whether consent controls are required in
   each launch jurisdiction; and
5. add a regression test covering the disclosed configuration.

## Contribution and ownership gaps

Repository history contains commits from Tushar-Upadhiya and automated Vercel
identities in addition to the two named owners. No signed assignment or
historical contributor agreement was found in the repository. Copyright should
not be represented as exclusively owned by Pratham Ranka and Ayush Garg until
qualified counsel reviews provenance and the applicable contributors provide
the required written license or assignment.

Future pull requests use the contribution grant and DCO sign-off in
`CONTRIBUTING.md`. If the business requires exclusive ownership, adopt a
counsel-approved copyright-assignment agreement before accepting more external
work; a DCO sign-off alone does not transfer copyright.

The seller verification packages and local MCP connector are currently marked
private/proprietary. Before distributing them to customers or package
registries, approve explicit customer-use and redistribution terms and produce
a complete third-party notice bundle for each artifact.

## Official GitHub references

- GitHub Terms of Service: https://docs.github.com/en/site-policy/github-terms/github-terms-of-service
- Licensing a repository: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/licensing-a-repository
- Setting repository visibility: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/managing-repository-settings/setting-repository-visibility
- Managing repository forking policy: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/managing-repository-settings/managing-the-forking-policy-for-your-repository
- About CODEOWNERS: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners
- About rulesets: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets
- Private vulnerability reporting: https://docs.github.com/en/code-security/security-advisories/working-with-repository-security-advisories/configuring-private-vulnerability-reporting-for-a-repository
