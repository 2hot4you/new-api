# Initial release review / checkpoint

Remote baseline origin/develop a71959f06630b8f4cc4792e0d3566825acc090cf contains the earlier rc.46 upgrade and successful deployment run39767173903 at source cf9b53e2a101e70ff743ec238fca662b0fc856ed. Origin main936aade39 is older. User explicitly authorizes current develop release promotion, not cherry-picking away that upgrade. Exact merged SHA must pass CI and dev live validation before main.

Existing worktree is isolated; separate develop/main worktrees remain untouched. Release branch release/molii-pricing-save-sync starts at current origin/develop and merges fix/molii-pricing-save-sync4898b8f55. Only conflict is additive .ccg/spec/guides/index.md; both upgrade acceptance and offline maintenance sections are preserved. App files auto-merge, with rc46 changes to upstream-price-cells retained. Default autostash failed without changing tracked files; merge was rerun with autostash disabled after confirming no tracked edits, rather than stashing user data.

External antigravity/Claude wrapper remains unavailable. Two native read-only reviewers used instead; no dual-external-model verification claimed. Workflow review: deploy push runs only application deploy for develop; main targets all four prod. Required deploy verify + actual release jobs must pass; the summary job may be green even if a target fails. Public health alone is insufficient: compare data.version target-<source SHA first12>, and preserve existing source/target catalog configuration. Do not replay obsolete runs or migrations blindly. Runtime env is preserved by existing deploy contract, not rewritten. No price sync/SQL maintenance in this release task.

Current phase: local merged-tree verification/review before push. No new deployment dispatched yet. Periodic checks must use exact source SHA and run IDs recorded in checkpoint.json and this thread; old successful runs are baseline only, not proof for new release.

## Pre-push acceptance

Both native independent reviewers found no app merge blocker. The only overlapping app file preserves both rc46 comparison highlighting and dedicated Seedance guards. Lead resolved the additive guides conflict by retaining both sections; git ls-files -u is empty and cached diff check passes. Merged-tree frozen install, 8 files/72 tests, typecheck, exact eight-file lint, i18n and Molii build all exit0 (see local-verification.md and fresh diagnostic logs). This is bounded local verification under Bun1.3.14; remote Bun1.4.0 full deploy verify is still mandatory. No app source was changed after these tests.

The first scheduler create request did not return and was stopped; local automation inventory contained no matching new task afterward. Do not claim scheduled follow-ups are enabled until a successful tool response confirms a heartbeat. A fresh purpose-built tool attempt will be made after develop push; never hand-write automation configuration or substitute OS cron.
