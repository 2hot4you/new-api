# Review

- Root cause: after rc.36 expanded the frontend suite, several complete jsdom router/dialog/mutation flows consistently take 5–6 seconds on the supported development runner. Vitest's default five-second per-test timeout caused different valid scenarios to fail depending on scheduling.
- Fix: set the shared Vitest `testTimeout` to 15 seconds and remove per-test timeout exceptions. Assertions, retry behavior, and production code are unchanged.
- `bun run test`: 206 files and 1,476 tests passed.
- `bun run typecheck`, changed-file lint, JSON validation, and diff checks passed.
