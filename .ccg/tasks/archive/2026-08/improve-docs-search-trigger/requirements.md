# Requirements

- Enable Algolia DocSearch only for Development when all three public build variables are present.
- Keep the existing local search for Production until separate Production credentials are supplied.
- Keep local development usable without Algolia variables.
- Reject partially configured Development credentials instead of silently deploying a broken search.
- Treat only the Search-only API key as public; never accept or document an Algolia Admin key.
- Preserve the existing `/docs/` base path, fixed light theme, navigation, and environment-aware Logo href.
- Wire repository Actions Variables into the documentation build without printing their values.
- Keep Development `noIndex` metadata; the Algolia crawler must use `ignoreNoIndex: true` for this non-production site.

## External model availability

Both required CCG analysis backends were invoked in parallel. The configured wrapper `/Users/naf/.claude/bin/codeagent-wrapper` is absent, so both calls exited 127 before analysis. Local analysis and TDD proceed with this limitation recorded for review.
