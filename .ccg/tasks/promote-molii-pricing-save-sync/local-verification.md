# Merged-tree local verification

Observed at 2026-10-22T12:24:19Z (20:24:19 Asia/Shanghai).

Scope: the current uncommitted merge on `release/molii-pricing-save-sync`, with HEAD `a71959f06630b8f4cc4792e0d3566825acc090cf` and MERGE_HEAD `4898b8f5512a76f55b6eafd9d1d741ad2ac51dd1`. The eight staged frontend files had merged cleanly; `.ccg/spec/guides/index.md` remained an unrelated unresolved documentation conflict during verification. No application source was changed by this verification. No push, server operation, data write, or full test-suite run was performed.

Local runtime: Bun **1.3.14** (`d1632b29`), Vitest **5.0.4**, Rsbuild **2.1.6**. The remote workflow uses Bun **1.4.0**; local success is not a substitute for its exact-source-SHA full gate.

## Results

All commands ran from `web/`; every command below exited **0**.

| Check | Command / result |
| --- | --- |
| Frozen dependencies | `bun install --frozen-lockfile`; final confirmation checked 1,244 installs across 1,340 packages, no changes |
| Related tests | `bun run test` with the eight explicit paths below: **8 files / 72 tests passed**, no skips reported, 16.12s, started 20:23:57 local |
| Types | `bun run typecheck` |
| Exact changed-file lint | `bunx --no-install oxlint -c .oxlintrc.json` with the eight explicit changed paths below; no diagnostics |
| Translation completeness | `bun run i18n:check`; 1 passed, 0 failed, 0 skipped |
| Molii production build | `VITE_SITE_PROFILE=molii bun run build`; built in 9.68s |

Related test paths:

- `src/features/model-pricing/__tests__/pricing.test.ts`
- `src/features/model-pricing/__tests__/editor-layout.test.tsx`
- `src/features/model-pricing/__tests__/editor-currency.test.tsx`
- `src/features/model-pricing/__tests__/save-errors.test.tsx`
- `src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx`
- `src/features/system-settings/models/__tests__/model-pricing-snapshots.test.ts`
- `src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx`
- `src/features/system-settings/__tests__/pricing-sync.test.tsx`

Exact changed lint paths:

- `src/features/model-pricing/__tests__/pricing.test.ts`
- `src/features/model-pricing/api.ts`
- `src/features/system-settings/__tests__/pricing-sync.test.tsx`
- `src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx`
- `src/features/system-settings/models/ratio-settings-card.tsx`
- `src/features/system-settings/models/upstream-price-cells.tsx`
- `src/features/system-settings/models/upstream-ratio-sync-table.tsx`
- `src/features/system-settings/models/upstream-ratio-sync.tsx`

Dependency fingerprints, unchanged before/after install and verification:

- `web/bun.lock`: SHA-256 `3c9dcd063c71a4c1730e1396056eb7e75c1b73e6c5482ee22e0af541e4f0030b`
- `web/package.json`: SHA-256 `8c665a86081fb4259b9916ef3c2ae862565f67802c0b7a1f09b1975a1041e75b`

Private local diagnostic logs: `/tmp/molii-release-local.gVousD/` (`install.log`, `install-final.log`, `pricing-tests.log`, `typecheck.log`, `lint.log`, `i18n.log`, `build.log`). These are not committed artifacts and are not evidence of any remote deployment.

## Remaining acceptance

Resolve the documentation merge conflict without changing verified application code, create the merge commit, then require the exact final develop source SHA's full remote verify/development release and live checks before main promotion. Main's new source SHA independently requires full verify and all four production releases/live checks. No local result establishes those outcomes.
