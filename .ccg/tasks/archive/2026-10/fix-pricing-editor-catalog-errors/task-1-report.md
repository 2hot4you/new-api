# Task 1 — pricing editor repair

## Scope and behavior

- Missing/unknown exact metadata or unknown source currency blocks a dirty ordinary pricing batch atomically before pricing PATCH or ExposeRatioEnabled PUT. Errors name all blocked models and retain the form/baseline. All four dedicated Seedance names retain existing exclusions.
- `isSaving` reflects only actual save mutations. Desktop, mobile sheet and embedded editors show an explicit prerequisite with `/models/metadata` opened in a new tab (`noopener noreferrer`), keeping the current draft. Unknown metadata is never treated as USD write eligibility.
- Add accepts an already-existing exact metadata row from the original full snapshot, with its stored currency and expected version. No metadata is created and no unrelated dirty model version is refreshed. Metadata created after loading still requires an explicit pricing reload.
- Local override deletion retains a visible save action after the selected/last editor closes. The menu says “Remove pricing override”; builtin-only rows explain the no-op. Row-menu events no longer bubble into the row edit handler and reopen a removed editor. Mutation failures retain local removal drafts.
- Reset policy: only persisted, non-Seedance overrides are candidates; builtin-only rows are excluded. Any metadata/currency blocker rejects the entire reset with named reasons. The confirmation explicitly describes this scope; embedded reset is disabled for ineligible/default-only rows.

## Skills/instructions

Read the task brief, requirements, frontend analysis, implementation plan, relevant guides spec, local shadcn skill/composition/forms/Base rules, and actual affected components. Used systematic-debugging, TDD and verification-before-completion. `shadcn info --json` confirmed Base UI/base-nova and existing primitives. Did not read AGENTS workflow guidance because the implementer role explicitly forbids it. No children, external models, dependency changes, backend changes, server actions, push or merge.

## RED evidence

Command (from `web`):

`bun run test src/features/model-pricing/__tests__/pricing.test.ts src/features/model-pricing/__tests__/metadata-prerequisite.test.tsx src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx`

Observed before production changes: **3 files failed; 12 failed / 14 passed; exit 1**. Failures covered missing/unknown metadata batch acceptance, desktop/mobile enabled save, embedded fake Saving, unknown Add name eligibility, missing-only/mixed JSON, reset blockers, builtin-only reset and deletion affordance.

After initial repair the deletion regression exposed the row-menu bubbling issue: removal reopened `Edit model pricing`; fixed the event boundary instead of weakening the assertion.

Additional targeted RED:

`bun run test src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx -t 'does not guess a source currency'`

Observed **1 failed / 12 skipped; exit 1**, enabled save for metadata with absent currency. Removed visual-editor USD fallback; the same test passes in the subsequent scoped suite.

## Verification

- Expanded focused suite (`model-pricing` plus ratio-settings-card, plugin-pricing, table-selection and table-columns): **9 files / 61 tests passed**, exit 0. Includes real mutation `Saving...`, failure/draft retention, original version/currency, Add eligibility, unknown metadata/currency, JSON atomicity, selected/last removal, default-only reset/removal, Seedance exclusions and provider drafts.
- `bun run test src/features/system-settings/__tests__/pricing-sync.test.tsx`: **15/15 pass**, exit 0.
- `bun run test src/features/models/__tests__/metadata-editing.test.tsx`: **9/9 pass**, exit 0.
- `bun run typecheck`: **pass**, exit 0 (fresh after application and integration fixture changes).
- Changed-file `oxlint -c .oxlintrc.json` over the explicit 14 files: **pass**, exit 0.
- `git diff --check` over owned/explicitly extended paths: **pass**.
- Final broader scoped suite: `bun run test src/features/model-pricing src/features/system-settings/models src/features/system-settings/__tests__/pricing-sync.test.tsx src/features/models/__tests__/metadata-editing.test.tsx` — **21 files / 170 tests passed**, exit 0, 24.68s. This additionally verifies the entire models-settings test directory. The missing-metadata JSON tests also toggle ExposeRatioEnabled and assert no PUT/PATCH/success message on rejection, with the real settings-page portal fixture.
- Full `bun run test`: **incomplete/failed, interrupted with exit 130** after approximately eight minutes; the sole remaining worker had been alive for over five minutes with no progress/output for several minutes after the Lobe failures below. Sent SIGINT only to this run’s verified PID 4705. No full-suite green claim. Full run was started during implementation, so it includes the subsequently corrected incomplete integration fixtures and intentional currency RED. Coordinator notified of each failure and must independently verify the stable final integration source.

### Full-suite findings and narrow scope extensions

The coordinator explicitly extended test-only ownership to two files after the full run exposed old incomplete/invalid fixtures:

1. `features/system-settings/__tests__/pricing-sync.test.tsx` — “previews the expression, rejects a stale save and reloads before applying without residual ratio differences”: exact model `m` fixture omitted `has_metadata`/currency. Added actual API fields `true`/`USD`, preserving stale-version rejection/reload assertions. Independent rerun 15/15 pass.
2. `features/models/__tests__/metadata-editing.test.tsx` — old “saves pricing and opens connections for a channel model without creating metadata” expected a write without exact metadata; replaced with “blocks pricing without exact metadata while preserving the draft and channel connections”, explicitly false metadata, no PATCH/PUT/POST, safe management link and input retention across tabs. Existing “keeps metadata drafts while saving pricing independently and preserves the price draft across tabs” uses real exact metadata fields true/USD; all independent draft/version/reload assertions retained. Independent rerun 9/9 pass.

The full run also collected the intentional currency RED above before its final fix. Scoped rerun passes.

Unrelated full-suite failures observed in `src/lib/__tests__/lobe-icon.test.tsx` (not modified):

- loads a named variant with its configured size and accessibility props
- updates the displayed icon when the name changes
- falls back to the base icon when the requested variant is unavailable
- loads the Gemma.Simple variant that is not listed in the standard catalog flags
- loads the LobeHub.Morden variant that is not listed in the standard catalog flags

All five report ~5-second loading timeouts. Existing full-run output also includes unimplemented jsdom scrollTo/canvas warnings and Base UI/reduced-motion warnings. Coordinator informed; integration owner must resolve/recheck the full-suite release gate.

The first final-scoped attempt also exposed a test setup error in the two JSON cases: the new visibility-atomicity assertion could not find ExposeRatioEnabled because the settings-page actions portal was unmounted. Added the real SettingsPageProvider/actions-container fixture (no production workaround), reran the complete 21-file scoped suite, and observed 170/170 pass. No failing test has been silently omitted from this report.

Broad directory lint (not the required changed-file gate) also found pre-existing errors in unchanged `models/upstream-ratio-sync-helpers.ts` (`prefer-string-replace-all`, `no-non-null-assertion`) and `models/utils.ts` (`prefer-at`, plus existing warnings). No unrelated fixes made. Task-created lint issues were fixed; explicit changed-file lint is clean.

## Exact new UI keys (all seven locales owned by Task 3)

1. `Pricing changes require exact model metadata and a known source currency: {{models}}. Create or verify metadata, then reload pricing. Your draft is preserved.` — `models`: comma-separated exact names (or existing translated “New model” placeholder).
2. `Manage model metadata`
3. `Remove pricing override`
4. `This model uses built-in defaults and has no pricing override to remove.`
5. `Built-in defaults are not removed. Save to apply override removals.`
6. `Reset eligible pricing overrides?`
7. `This clears persisted overrides with exact metadata only. Built-in defaults and dedicated Seedance pricing are unchanged. If any override is ineligible, nothing is reset.`

No other new translation keys. Keys sent to coordinator before locale integration.

## Files changed

- web/src/features/model-pricing/api.ts
- web/src/features/model-pricing/model-pricing-panel.tsx
- web/src/features/model-pricing/__tests__/pricing.test.ts
- web/src/features/model-pricing/__tests__/metadata-prerequisite.test.tsx (new)
- web/src/features/model-pricing/__tests__/editor-layout.test.tsx
- web/src/features/system-settings/models/model-pricing-sheet.tsx
- web/src/features/system-settings/models/model-ratio-form.tsx
- web/src/features/system-settings/models/model-ratio-visual-editor.tsx
- web/src/features/system-settings/models/model-ratio-table-columns.tsx
- web/src/features/system-settings/models/ratio-settings-card.tsx
- web/src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx
- web/src/features/system-settings/models/__tests__/plugin-pricing.test.tsx
- web/src/features/system-settings/__tests__/pricing-sync.test.tsx (explicit test-only extension)
- web/src/features/models/__tests__/metadata-editing.test.tsx (explicit test-only extension)
- This requested report (not staged with application commit).

## Commit

`559c9074f74af6881c7d8130ede0103cb454dc9c` — `fix: enforce pricing metadata eligibility and preserve override drafts`

Explicitly staged only the 14 application/test files listed above after fresh scoped tests, typecheck, changed-file lint and diff-check. `git show --format= --name-only HEAD` confirmed exactly those paths. Other workers’ catalog/backend/locale/workflow changes were not staged. This requested report remains outside the application commit for coordinator integration. No push/merge performed. Ready for independent coordinator review; full-suite release gate is still outstanding as described above.
