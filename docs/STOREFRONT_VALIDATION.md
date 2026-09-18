# Storefront discovery validation contract

Status: **Locked for SEO-003**.

AgentPay validates generated storefront artifacts deterministically before a
seller treats them as publication-ready. Validation reports named checks and
issues only. It never predicts, promises, or reports search ranking, traffic,
or conversion.

## MCP operation

`validate_storefront_artifacts` is a read-only MCP tool requiring the
`validate` scope. The request is limited to 100 pages and 1 MiB of allowlisted
artifact facts. It accepts no repository secrets, source files, environment
values, customer records, deployment credentials, or arbitrary URLs to fetch.

The caller supplies the canonical HTTPS storefront base URL, generated page
facts, `robots.txt`, `sitemap.xml`, `llms.txt`, and the AgentPay manifest JSON.
Page facts include visible title, description, heading, text, canonical URL,
robots directive, JSON-LD, accessibility counts, byte measurements, blocking
script count, and measured largest contentful paint.

## Required checks

The result contains exactly these ordered checks:

1. `metadata`: unique titles of 10-60 characters and descriptions of 50-160
   characters;
2. `canonical_urls`: absolute HTTPS, same-origin, self-canonical page URLs;
3. `robots_directives`: index/follow page metadata, no storefront-wide block,
   and a canonical sitemap declaration;
4. `sitemap_output`: valid XML containing every canonical product URL;
5. `structured_data`: valid Schema.org Product or Service JSON-LD whose name,
   description, and URL match visible page facts, with no generated ratings or
   reviews;
6. `semantic_content`: one title-matching H1, at least 100 visible characters,
   and no guaranteed-ranking claims;
7. `llms_text`: every page's title, description, and canonical URL are present;
8. `manifest_consistency`: enabled paid-route paths and descriptions match the
   generated product pages exactly;
9. `accessibility`: document language, exactly one main landmark, no missing
   informative-image alt text, and no unlabelled controls; and
10. `performance_budgets`: per-page HTML at most 200 KB, JavaScript at most
    250 KB, CSS at most 100 KB, at most two blocking scripts, and measured LCP
    at most 2,500 ms.

All checks must pass for `valid=true`. There is no weighted score and no
override that converts a failing artifact set into a passing result.
