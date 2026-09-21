# Task 4 Report: Claudeye brand appearance editor

## Outcome

Implemented the Claudeye-only root-admin brand appearance section using the existing system option form and mutation infrastructure. The editor manages exactly the four approved `brand_setting.*` values, keeps native color controls and editable HEX values synchronized, retains the last valid preview during partial input, warns without blocking on low contrast, falls back to the approved neutral wordmark, restores defaults without saving, and saves changed values sequentially through the existing option mutation in uppercase form.

## TDD evidence

### RED: pure helpers

Command:

```bash
cd web
bun test src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts
```

Observed expected failure before helper implementation:

```text
error: Cannot find module '../claudeye-brand-colors'
0 pass
1 fail
1 error
```

### GREEN: pure helpers

The same command passed after the minimal helper implementation:

```text
4 pass
0 fail
10 expect() calls
```

### RED: section interactions

Command:

```bash
cd web
bun test src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx
```

Observed expected failure before section implementation:

```text
error: Cannot find module '../claudeye-brand-appearance-section'
0 pass
1 fail
1 error
```

### GREEN: required focused suite

Command:

```bash
cd web
bun test src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts \
  src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx
```

Final result:

```text
12 pass
0 fail
48 expect() calls
Ran 12 tests across 2 files.
```

The interaction coverage includes all four pickers/text inputs/swatches, bidirectional synchronization, last-valid previews, neutral-image fallback, invalid input validation and save blocking, warning-only low contrast, restore-default dirty behavior, uppercase option payloads, and Molii/iXiaozu absence plus Claudeye presence in the registry.

## Verification

### Required commands

```bash
cd web
bun run i18n:check
```

Result: PASS, 1 test passed and 0 failed.

```bash
cd web
bun run typecheck
```

Result: PASS (`tsgo -b`, exit 0).

```bash
cd web
bun run lint
```

Result: FAIL on the repository's existing lint backlog outside the Task 4-owned files. Representative findings include `src/features/redemption-codes/components/redemptions-provider.tsx`, `src/lib/utils.ts`, `src/components/confirm-dialog.tsx`, and existing layout/assets files. No Task 4-owned file was reported in the final run.

Focused validation of every owned TypeScript/TSX file:

```bash
cd web
bunx oxlint -c .oxlintrc.json \
  src/features/system-settings/site/claudeye-brand-colors.ts \
  src/features/system-settings/site/claudeye-brand-appearance-section.tsx \
  src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts \
  src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx \
  src/features/system-settings/types.ts \
  src/features/system-settings/site/index.tsx \
  src/features/system-settings/site/section-registry.tsx
```

Result: PASS, exit 0 with no output.

Formatting validation over all owned source and locale files also passed with `All matched files use the correct format.`

### Wider regression tests

Configured system-settings suite:

```bash
cd web
bun run test -- src/features/system-settings
```

Result:

```text
Test Files  12 passed (12)
Tests       45 passed (45)
```

Configured full frontend suite:

```bash
cd web
bun run test
```

Result:

```text
Test Files  219 passed (219)
Tests       1532 passed (1532)
```

A direct recursive `bun test src/features/system-settings` run was also attempted. It is not a stable wider-suite invocation because legacy test files share and close manually installed DOM globals; it failed in unrelated pricing/model tests and caused cross-file DOM interference. The configured Vitest settings suite above passes all 45 tests, including Task 4.

## Staged-file scope

The commit contains only the brief-owned implementation/test/locale files and this report:

- `web/src/features/system-settings/site/claudeye-brand-colors.ts`
- `web/src/features/system-settings/site/claudeye-brand-appearance-section.tsx`
- `web/src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts`
- `web/src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx`
- `web/src/features/system-settings/types.ts`
- `web/src/features/system-settings/site/index.tsx`
- `web/src/features/system-settings/site/section-registry.tsx`
- `web/src/i18n/locales/en.json`
- `web/src/i18n/locales/zh.json`
- `web/src/i18n/locales/zh-TW.json`
- `web/src/i18n/locales/ja.json`
- `web/src/i18n/locales/fr.json`
- `web/src/i18n/locales/ru.json`
- `web/src/i18n/locales/vi.json`
- `.superpowers/sdd/2026-09-21-claudeye-runtime-brand-colors/task-4-report.md`

## Self-review

- Confirmed the section is constructed only when `SITE_BRAND.id === 'claudeye'`; Molii and iXiaozu return no brand appearance descriptor.
- Confirmed no new route, endpoint, permission, dependency, or storage key was introduced.
- Confirmed saves use `useUpdateOption().mutateAsync` sequentially with exact flattened option keys and normalized uppercase values.
- Confirmed invalid/partial values never enter preview URLs, keep the prior valid preview and picker/swatch color, expose a translated format error, and disable Save.
- Confirmed contrast is calculated with WCAG relative luminance against `#FFFFFF` and `#171717`; low contrast is advisory only.
- Confirmed preview query construction uses `URLSearchParams` and preview image errors switch once to `/claudeye-wordmark-neutral.png`.
- Confirmed restore defaults changes form values and dirtiness only; mutation calls occur only after the main Save action.
- Confirmed all ten required translation keys exist in all seven locale files and `zh.json` uses the approved design wording.
- Confirmed `git diff --check` and focused lint/format checks are clean.

## Concerns

- Repository-wide lint remains blocked by pre-existing findings outside Task 4 ownership. Task-local lint is clean.
- No browser-level visual QA was required for this delegated task; interaction and rendering contracts are covered in automated DOM tests.

## Review fix round 1

### Outcome

Addressed both Important review findings without changing the approved per-option API. An HTTP 200 option response with `success: false` now stops the sequential save immediately and prevents `useSettingsForm` from resetting its saved baseline, so all attempted values remain dirty and available for retry. The existing `useUpdateOption` unsuccessful-response handler remains responsible for the visible error toast. Each surface now derives its displayed palette from the actual watched form values whenever both values are valid and stores that complete palette for preview, picker, swatch, and contrast fallback during later partial input.

### RED evidence

Added three regression tests before changing the implementation, then ran:

```bash
cd web
bun test src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts \
  src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx
```

Expected failures reproduced both findings:

```text
12 pass
3 fail
78 expect() calls
```

- First-field business failure: expected the attempted `#abcdef` to remain, but the form reset it to `#111111`.
- Later business failure: expected the sequential loop to stop after 2 calls, but it made 3 calls.
- Options refresh: expected `surface=light&mark=%23FFFFFF&text=%23222222`, but the preview remained on `%23111111`.

The failure tests also cover retrying the first failed value, preserving all three edits when the second option fails after the first succeeds, and stopping before the third option.

### GREEN evidence

The same focused command after the minimal implementation fix:

```text
15 pass
0 fail
74 expect() calls
Ran 15 tests across 2 files.
```

The refresh regression verifies the refreshed valid input (`#FFFFFF`), preview URL, native picker, visible swatch, and low-contrast warning. It then enters partial `#FFF` and verifies that all four surfaces retain the refreshed complete palette rather than the mount-time palette.

### Review-fix verification

```bash
cd web
bun run test -- src/features/system-settings
```

```text
Test Files  12 passed (12)
Tests       48 passed (48)
```

```bash
cd web
bun run test
```

```text
Test Files  219 passed (219)
Tests       1535 passed (1535)
```

```bash
cd web
bun run i18n:check
bun run typecheck
```

Both passed: locale completeness reported 1 passing test; `tsgo -b` exited 0. An intermediate typecheck correctly caught two test-only `.src` accesses typed as `HTMLElement`; the assertions were narrowed to `HTMLImageElement`, and the final typecheck passed.

```bash
cd web
bunx oxlint -c .oxlintrc.json \
  src/features/system-settings/site/claudeye-brand-colors.ts \
  src/features/system-settings/site/claudeye-brand-appearance-section.tsx \
  src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts \
  src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx \
  src/features/system-settings/types.ts \
  src/features/system-settings/site/index.tsx \
  src/features/system-settings/site/section-registry.tsx
```

Result: PASS, exit 0 with no output. Focused `oxfmt --check` on the two changed source/test files also passed, and `git diff --check` passed.

```bash
cd web
bun run lint
```

Result: FAIL on the existing repository-wide lint backlog outside Task 4 ownership. Representative unchanged findings remain in `src/lib/utils.ts`, `src/features/redemption-codes/components/redemptions-provider.tsx`, and `src/components/confirm-dialog.tsx`; task-local lint is clean.

### Review-fix self-review and concerns

- Confirmed an unsuccessful response is checked before the next mutation and before the shared form hook can commit/reset its baseline.
- Confirmed both response-level failures and transport failures continue through the existing mutation hook error UI; the local submit boundary only consumes the rejected control-flow promise to avoid an unhandled event-handler rejection.
- Confirmed valid watched values immediately drive preview and contrast, while the synchronized last-valid surface palette supplies invalid-input fallback to preview, picker, and swatch.
- Confirmed the diff from `b111b6c9272390f923168648d841fcd20240e924` changes only the owned section implementation, its interaction test, and this report.
- With accepted sequential semantics, retrying after a later failure replays an earlier successful option because the complete form remains dirty. Option updates are idempotent, and preserving the full unsaved form is required; no atomic endpoint was added.

## Review fix round 2

### Root cause and correction

The round-1 tests did not mount an active `system-options` query. In the real settings flow, every successful `useUpdateOption` mutation invalidates that query. A refetch after the first successful option can therefore publish a partially updated `defaultValues` object while the next sequential mutation is pending. The shared `useSettingsForm` treats changed defaults as authoritative and resets the form, erasing the failed and unsent edits before the later `{ success: false }` response is handled.

The section now keeps a stable accepted-defaults snapshot for `useSettingsForm`. Incoming defaults are not accepted while the form is dirty or submitting. After a completely successful submission, the section waits until refreshed defaults contain the values of every key that submission saved before accepting them; this prevents a stale partial refetch from winning. The guard compares only saved keys, so clean forms can still absorb legitimate concurrent changes to untouched options. Failed submissions never establish a new saved-key guard and remain dirty for retry.

### RED evidence

Added a query-connected interaction harness using the real `useSystemOptions`, `getOptionValue`, `useUpdateOption`, query invalidation, refetch, and form hooks. Only the API boundary is mocked.

Command:

```bash
cd web
bun test src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx \
  --test-name-pattern "retains failed and unsent edits when an earlier save refetches system options"
```

Before the correction, the regression failed after the second mutation returned `{ success: false }`:

```text
Expected: save.disabled false
Received: save.disabled true
0 pass
1 fail
25 expect() calls
```

This reproduced the reviewer result: the first successful mutation caused an actual second options GET, the failed and unsent inputs reset, and retry was unavailable.

A second RED refinement changed an untouched server option during retry to verify that the stale-response guard does not freeze legitimate clean refreshes. A full-palette guard failed because refreshed defaults were never accepted:

```text
Expected: light mark #ABCDEF
Received: light mark #abcdef
0 pass
1 fail
50 expect() calls
```

The guard was then narrowed to only the keys saved by the completed submission.

### GREEN evidence

The query-connected regression passed after the narrow fix:

```text
1 pass
0 fail
20 expect() calls
```

It verifies that:

- The earlier successful mutation triggers a real query invalidation and refetch.
- The later unsuccessful response leaves the failed and unsent values intact and Save enabled.
- Only two original mutations are sent; the third is not sent after failure.
- Retry replays the retained values and persists all three requested changes.
- The completed form absorbs a concurrent refreshed value for the untouched fourth option.

Final focused command:

```bash
cd web
bun test src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts \
  src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx
```

```text
16 pass
0 fail
94 expect() calls
Ran 16 tests across 2 files.
```

The existing options-refresh/picker/swatch/preview/contrast regression remains green, preserving the round-1 Important 2 fix.

### Review-fix verification

```bash
cd web
bun run test -- src/features/system-settings
```

```text
Test Files  12 passed (12)
Tests       49 passed (49)
```

```bash
cd web
bun run test
```

```text
Test Files  219 passed (219)
Tests       1536 passed (1536)
```

`bun run i18n:check` passed its locale-completeness test. `bun run typecheck` passed with `tsgo -b` exit 0. An intermediate typecheck caught an implicit `any` in the new API-boundary test double; the request was typed from `updateSystemOption`, and the final typecheck passed.

Task-local `oxlint` over all Task 4-owned TypeScript/TSX files passed with exit 0. Focused `oxfmt --check` on the changed implementation and test passed. `git diff --check` passed.

`bun run lint` still exits 1 on the existing repository-wide backlog outside Task 4 ownership, including unchanged findings in `src/features/redemption-codes/components/redemptions-provider.tsx`, `src/lib/utils.ts`, and `src/components/confirm-dialog.tsx`. No Task 4-owned file is reported.

### Review-fix self-review and concerns

- The fix is local to the Claudeye section; the shared settings form hook and backend option API are unchanged.
- Incoming defaults remain responsive on clean forms, as covered both by the original refresh/contrast test and the query-connected concurrent untouched-option refresh.
- Dirty/submitting forms retain their complete local values across any intermediate options refetch.
- Successful completion accepts refreshed defaults only after all keys saved by that submission match, preventing stale partial query results from replacing the just-saved baseline.
- Sequential retry intentionally resends an earlier successful idempotent option because all attempted values remain dirty after a later failure.
- No remaining Task 4-specific blocker was found. Repository-wide lint remains the only known failing command and is outside the owned scope.
