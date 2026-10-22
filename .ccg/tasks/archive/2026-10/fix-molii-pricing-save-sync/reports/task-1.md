# Task 1 — Generic upstream selection boundary

## Changes

- `web/src/features/system-settings/models/upstream-price-cells.tsx`: use the existing `isSeedanceManagedPricingModel` predicate. Dedicated models show the existing translated Seedance-tab explanation and unchanged upstream price display, without a generic selection control.
- `web/src/features/system-settings/models/upstream-ratio-sync-table.tsx`: exclude dedicated models from filtered, cross-page bulk selections and eligible/selected counts. Dedicated differences remain visible for explanation, not silently hidden or marked imported.
- `web/src/features/system-settings/models/upstream-ratio-sync.tsx`: filter protected models at incoming-selection and resolution boundaries; derive selection counts and apply eligibility from valid resolutions; preview only valid resolutions and reject an empty save. Success updates only those eligible resolutions.
- `web/src/features/system-settings/__tests__/pricing-sync.test.tsx`: all four dedicated names, stale table selections, protected-only disabled bulk action, mixed and filtered bulk counts, mobile comparison behavior, and mixed bulk confirmation/save payload exclusion. The existing optimistic-version conflict/reload and expression-preservation assertions remain covered.

No locale, expression normalization, shared API, reset, server, commit, or push changes. Existing comparison, selection labels, native keyboard/focus behavior, confirmation and reusable table/cell components remain in use; no new common UI implementation was introduced.

## TDD evidence

Before production changes, ran from `web/`:

`bun run test src/features/system-settings/__tests__/pricing-sync.test.tsx`

RED: 3 failed / 12 passed. Failures directly exposed protected individual checkboxes, a mixed bulk count of five instead of one, and a selected-model count including dedicated models.

After implementation and strengthened mobile/stale fixtures, the same focused suite passed 15 / 15. Final rerun evidence is listed below.

## Verification

- Focused test suite: final run passed 15 / 15 (7.58 seconds); also asserted the remaining dedicated-only differences leave Apply Sync disabled after an ordinary save.
- Changed-file `bunx oxlint -c .oxlintrc.json` across the four owned files: passed on the final run, no diagnostics.
- `bun run typecheck`: final rerun passed (exit 0). An earlier concurrent run briefly encountered Task 2 test typing errors outside ownership; they were reported to the lead and repaired by their owner before this passing rerun.
- `git diff --check`: passed.
- Full frontend suite: lead explicitly owns the integrated full-suite run; not duplicated here.

## Boundary notes

The stale-selection regression directly exercises stale incoming table props. The component's internal resolutions additionally check the predicate before building any selected pricing bundle, and the apply preview is restricted to those resolutions. The shared generic change-builder protection is intentionally owned by Task 2.
