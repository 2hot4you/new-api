# China Holiday Billing Calendar Implementation Plan

1. Add failing Go tests for holiday/rest/makeup/unknown-year/timezone behavior, request tracing, and cached-program clock freshness.
2. Add the versioned calendar JSON with an explicit 2026-10-01 coverage start, embed/validate it, expose classification metadata, and register `is_holiday` in compile/runtime environments.
3. Add failing Web expression tests, then implement the two-string boolean function across parser, runtime, trace detection, display, refresh detection, and visual round-trip.
4. Add a read-only AdminAuth endpoint returning expression-priced model data plus calendar metadata.
5. Add route, sidebar entry, model selector, semantic month calendar, selected-day minute-precision interval detail, and selected-model/current-month Apple Calendar export with tests.
6. Run focused tests, Go suite, Web suite, typecheck, build, route generation, and formatting checks.
7. Review the complete diff, fix findings, archive the CCG task, commit, push HEAD to `origin/develop`, and monitor deployment.
