# Review

## Scope

- Development builds use Algolia DocSearch only when all three public repository Variables are present.
- Production and local builds without those Variables retain the existing local search implementation.
- The Development Search-only key is never read from `secrets.*`, logged, or returned by a server API.

## Findings

- Critical: none.
- Warning: Development remains `noIndex`; the Algolia crawler must set `ignoreNoIndex: true` or it will intentionally skip the pages.
- Info: a partially configured Development build fails with the names of the missing configuration group, without printing values.
- Info: Production ignores Algolia values even if they are accidentally present in the process environment.

## External review availability

Both CCG analysis and review calls were launched in parallel for antigravity and Claude. All four calls exited 127 because `/Users/naf/.claude/bin/codeagent-wrapper` is not installed. No external-model findings were available; local security and behavior review was completed instead.

## Verification

- RED: configuration and workflow tests failed before the public Algolia allowlist, conditional configuration, and Actions Variables wiring existed.
- Focused configuration/workflow tests: 15 passed.
- Development Algolia browser contract: 13 passed.
- Production local-search browser contract with Development-like variables present: 13 passed.
- Full Development `bun run check`: 129 tests passed; forbidden-term scan, secretlint, Docusaurus `/docs/` build, and 30-link crawl passed.
- `bun run api:lint`: valid.
- `bun run catalog:check`: matched.
- Workflow YAML parsed successfully.
- Production build contained no Development Algolia application ID and generated the local-search index.
- `git diff --check`: passed.
