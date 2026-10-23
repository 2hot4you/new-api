# Stable-source verification and release boundary

Application/test source reviewed: `915723524..9a58dffae`; later commits before release contain task records only. Both independent final reviews pass without actionable findings. The unavailable external wrapper was replaced by native independent reviews, not represented as antigravity/Claude output.

## Observed local verification

Evidence directory: `/tmp/molii-pricing-repair-verify.Wdi5yR/`.

- Fresh worktree-owned frozen-lock frontend install; no package or lockfile edits. The old shared node_modules symlink is recoverably preserved at `original-node-modules-link` in that evidence directory. No shared checkout dependencies were modified.
- Full frontend `bun run test`: 328 files, 3082 tests passed, exit 0 (`frontend-full.log`).
- Frontend typecheck, i18n completeness and production build passed; site-brand profiles 7/7 passed. All changed-file formatting/lint checks passed. Broad-directory lint/source scans still have disclosed pre-existing findings outside this repair; no whole-repository lint-clean claim.
- Root Go vet/build passed. Independent relaykit and tokenkit vet/build passed, all with GOWORK=off.
- `make test-postgres`: exit 0. Full database-independent root 43 packages/2685 tests, relaykit 9 packages/568 tests, tokenkit 1 package/7 tests, all zero skips. Mandatory PostgreSQL inventory: 1660 tests/subtests across 40 exact selections, zero skips (`postgres-normal-final.log`).
- `make test-postgres-race`: exit 0. Full database-independent root, relaykit and tokenkit suites passed with zero skips; mandatory PostgreSQL inventory: 1660 tests/subtests across 40 exact selections, zero skips and no reported data race (`postgres-race-final.log`).
- Deployment contracts: Python 29/29 and shell 192/192 assertions passed, exit 0 (`deployment-contracts.log`).
- Task-level regression-first evidence and independent findings are in the adjacent task reports/reviews. The initial interrupted frontend run and original DDL scheduling failure are not accepted green runs; the final successful runs supersede them after narrowly reviewed fixture fixes.

Coverage is the required PostgreSQL and independent-package acceptance, not historical broad cross-engine `make test`. No production SQL/schema/billing acceptance or quote conversion was changed. Disposable fixtures used owned PostgreSQL/Redis containers bound to loopback ports 32770/32771, never production data.

Final verification completed 2026-10-23 Asia/Shanghai. Both normal/race runner sessions completed with exit 0 on the reviewed application tree. Subsequent staged edits are task reports and guide feedback only; `git diff --check` passes. No implementation finding remains open. GitHub-hosted acceptance is still required independently.

## Remaining release acceptance

Push the fully verified exact source to actual `develop`, not an invented dev branch. Inspect that source's GitHub Actions verify and Release development jobs/attempts, then verify `https://dev.molii.co/api/status` success and exact `development-<source SHA first 12>` version. Existing successful runs for 915723524 are not acceptance for this repair. Fresh fetch must exclude unreviewed intervening changes before promoting that exact accepted source to main. Do not create a different main source SHA solely for audit records between development acceptance and promotion.

The source fixed-Grok/xAI task-usage conflict intentionally remains rejected. This repair makes it actionable and safe; it does not authorize live pricing changes, catalog apply, plugin disabling, auto metadata creation, quote deletion or conversion.
