# Review

## Verdict

Approved. The documentation favicon now uses an exact byte-for-byte copy of the New API browser favicon at a new, versioned documentation path.

## Scope and safety

- The navbar wordmark, social preview image, application favicon, deployment configuration, and runtime behavior are unchanged.
- The new contract test prevents the documentation copy from diverging from `web/public/molii-favicon-32.png`.
- The `/docs/` production build emits `/docs/img/molii-favicon-32.png?v=4`, and the link crawler confirms that URL returns HTTP 200.
- No secrets or environment values were added.

## Verification

- `bun test`: 130 passed, 0 failed.
- Development-shaped `bun run build`: passed.
- `bun run check:forbidden`: passed.
- `bun run check:secrets`: passed.
- `DOCS_BASE_URL=/docs/ bun run check:links`: 29 links passed.
- `git diff --check`: passed.

The first unscoped link-check invocation used a `/docs/` build with the default `/` crawler base and therefore produced expected 404s. Re-running with the matching `DOCS_BASE_URL=/docs/` deployment contract passed.
