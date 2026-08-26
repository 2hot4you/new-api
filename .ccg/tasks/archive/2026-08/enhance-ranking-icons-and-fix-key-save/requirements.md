# Requirements

## Rankings icons

- Show each configured model icon in the Top Models chart tooltip.
- Show each configured vendor icon in the Market Share chart tooltip.
- Show vendor icons in the "By model author" list.
- Reuse `getLobeIcon`; missing or invalid icon keys must fall back safely.
- Do not change ranking order, metric calculations, or chart colours.

## API key save regression

- Creating or updating an API key must not crash the React tree after a successful response.
- Preserve existing IP/CIDR validation, canonicalisation, model restrictions, group routing, payload formats, and success toasts.
- Add a regression test that closes the real Sheet after a successful create/update and catches repeated state updates.

## Delivery

- Work only on `develop` in its existing worktree.
- Run focused tests, typecheck, scoped lint, i18n check, frontend build, backend tests/vet/build.
- Review, archive the CCG task, commit, push `origin/develop`, and verify Development deployment.
