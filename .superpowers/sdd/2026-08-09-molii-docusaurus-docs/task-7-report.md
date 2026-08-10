# Task 7 report: help, changelog, local search, and quality gates

## Delivered

- Replaced the placeholder changelog with the requested MDX page, retaining the single `/changelog` route.
- Added user-facing troubleshooting and support-contact guides without exposing credentials, private data, or operator-only surfaces.
- Configured Chinese local search with `language: ['zh']` and the `@node-rs/jieba` dependency expected by the installed Lunr Chinese tokenizer. The README documents that an index is created by production build and is tested with `bun run preview`.
- Added a focused forbidden-content scanner with precise `file:line` output. It rejects user-documentation paths and terminology that expose non-public operation surfaces, internal domains, and realistic secrets while allowing documented placeholders and the New API / QuantumNous attribution.
- Added Secretlint configuration, a separate optional external-link command, and a deterministic internal-link command that serves the already-built static site locally before crawling it.
- Added a static-only Nginx example with SPA fallback and immutable `/assets/` caching. It has no certificate, upload, container, credential, or deployment automation configuration.

## RED / GREEN

| Stage | Command | Result |
| --- | --- | --- |
| RED | `cd docs-site && bun test scripts/check-forbidden-terms.test.ts` | Expected failure: scanner module did not exist. |
| GREEN | `cd docs-site && bun test scripts/check-forbidden-terms.test.ts` | Pass: 2 tests, including exact location reporting and safe placeholders/attribution. |
| Focused gates | `bun run check:forbidden && bun run check:secrets` | Pass. |
| Full gate | `bun run check` | Pass: 66 tests, 0 failures; forbidden-content, Secretlint, production build, and 51-link deterministic local crawl all pass. |
| Frozen install | `bun install --frozen-lockfile` | Pass: no lockfile changes. |
| Explicit build | `bun run build` | Pass: static site and two local-search indexes generated. |
| Diff check | `git diff --check` | Pass. |

## Files changed

- `docs-site/.secretlintignore`
- `docs-site/.secretlintrc.json`
- `docs-site/README.md`
- `docs-site/bun.lock`
- `docs-site/docs/changelog.mdx`
- `docs-site/docs/changelog.md` (removed, superseded by the MDX page)
- `docs-site/docs/help/troubleshooting.mdx`
- `docs-site/docs/help/contact-support.mdx`
- `docs-site/docusaurus.config.ts`
- `docs-site/examples/nginx.conf.example`
- `docs-site/package.json`
- `docs-site/quality/forbidden-terms.txt`
- `docs-site/scripts/check-forbidden-terms.mjs`
- `docs-site/scripts/check-forbidden-terms.test.ts`

## Notes

- `check:links:external` remains intentionally outside the default `check` gate because it depends on external network state.
- The required production build and Chinese local-search index generation both pass with `@node-rs/jieba@2.0.1`; no fallback tokenizer warning is emitted.
- No commit was created, per handoff instruction.
