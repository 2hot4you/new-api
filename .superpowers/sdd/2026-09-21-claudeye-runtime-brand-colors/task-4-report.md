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
