# Review

- Root cause: the existing metadata/pricing drawer integration test performs multiple complete tab, mutation-conflict, reload, and retry flows and consistently takes about 5.9 seconds in the merged full suite, exceeding Vitest's default 5-second per-test budget.
- Fix: apply a 15-second timeout only to this test. No production behavior or assertion was changed, and the global timeout remains unchanged.
- Focused verification: `metadata-editing.test.tsx` passed 9/9.
- Full verification: `bun run test` passed 206 files and 1,476 tests.
- Changed-file lint and diff checks passed.
