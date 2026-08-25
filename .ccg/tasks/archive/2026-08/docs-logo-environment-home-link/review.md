# Review

## Scope

- The Docusaurus wordmark uses the validated public site origin already supplied by each deployment environment.
- The link stays in the current tab.
- No search provider, deployment secret, or unrelated navigation behavior changed.

## Findings

- Critical: none.
- Warning: none.
- Info: local development follows `DOCS_SITE_URL`, matching the same explicit configuration contract as Development and Production.

## Verification

- RED: `bun test src/config.test.ts` failed because the existing href was `/quick-start` and no target was configured.
- GREEN: `bun test src/config.test.ts scripts/default-theme-contract.test.ts` — 15 passed.
- Development browser contract with `DOCS_SITE_URL=https://dev.molii.co` and `DOCS_BASE_URL=/docs/` — 13 passed.
- Full `bun run check` — 126 tests passed; forbidden-term, secretlint, build, and 29-link crawl passed.
- `bun run api:lint` — valid.
- `bun run catalog:check` — snapshot matched.
- `git diff --check` — passed.
