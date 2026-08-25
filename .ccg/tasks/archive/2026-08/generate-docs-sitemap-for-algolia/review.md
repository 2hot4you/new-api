# Review

## Scope

- `docs-site/package.json`
- `docs-site/scripts/generate-sitemap.mjs`
- `docs-site/scripts/generate-sitemap.test.ts`

## Findings

- Critical: 0
- Warning: 0
- Info: Development Sitemap intentionally lists public documentation routes while the pages retain `noindex, nofollow`; this permits the configured Algolia crawler to discover pages without allowing ordinary search engines to index the Development environment.

## Correctness and safety

- The generator derives entries only from HTML files actually emitted into `build/`.
- The server-owned `/docs/` redirect page, `404.html`, static assets, and Docusaurus internal routes are excluded.
- The environment-specific `siteUrl` and `baseUrl` from the validated Docusaurus configuration determine every URL.
- XML-sensitive characters are escaped and entries are sorted deterministically.
- The script reads filenames only; it does not read page content, credentials, or runtime configuration secrets.
- Production remains indexable; Development retains its existing `noindex, nofollow` contract.

## Verification evidence

- TDD RED: the focused tests failed because `sitemap.xml` was absent.
- TDD RED for XML safety: the focused test failed on an unescaped `&`.
- Focused GREEN: 2 tests, 5 assertions.
- Non-browser documentation suite: 110 tests, 1,756 assertions.
- Development build: 95 Sitemap URLs under `https://dev.molii.co/docs/`; quick-start retained `noindex, nofollow`.
- Production build: 95 Sitemap URLs under `https://molii.co/docs/`; quick-start had no `noindex` directive.
- `xmllint --noout`: passed.
- Forbidden-content, secretlint, OpenAPI lint, catalog consistency, and `/docs/` link crawl: passed.

## CCG external review availability

The required `~/.claude/bin/codeagent-wrapper` executable is absent in this environment, and no callable antigravity or Claude review tool is installed. The mandatory external dual-model analysis/review could not be executed; the local evidence and review above are recorded instead.
