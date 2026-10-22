# Task 2 — Generic save/reset defensive boundary

## Owned changes

- `web/src/features/model-pricing/api.ts`: `buildPricingChanges` skips all existing dedicated Seedance model identities before constructing generic changes. Ordinary expected versions, configured-field preservation, explicit zero, currency overrides/defaults and missing-metadata handling stay unchanged.
- `web/src/features/model-pricing/__tests__/pricing.test.ts`: mixed dedicated/ordinary additions plus protected currency-only changes; verifies all four dedicated names, ordinary versions/currency/zero and missing-metadata safety.
- `web/src/features/system-settings/models/ratio-settings-card.tsx`: reset filters dedicated entries and avoids save/refetch on an empty eligible batch; no false reset-success toast. Generic JSON save detects dedicated field/provider/currency edits with existing parsing/equality helpers and shows the existing translated dedicated-tab guidance. Ordinary edits in mixed saves still persist; skipped edits are not written into the saved snapshot or reported as full success. Canonical refetch restores actual protected values after a mixed save.
- `web/src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx`: one new real-component fixture because existing group-settings tests exercise a different form. Mocks only API transport, uses real pricing hooks, card, JSON editor, confirmation dialog and toast renderer. Covers mixed save, protected-only save, mixed reset and protected-only reset.

## RED evidence

From `web/`:

`bun run test src/features/model-pricing/__tests__/pricing.test.ts src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx`

Before implementation: exit 1, 5 failed / 9 passed. Builder emitted four forbidden dedicated changes. Mixed JSON PATCH included the dedicated model. Protected-only JSON save omitted guidance and falsely saved. Both reset cases included dedicated entries / failed the no-change notice expectation. No setup/runtime errors prevented these behavior assertions.

## GREEN and checks

- First post-fix targeted run: same command, exit 0, 2 files / 14 tests passed.
- Initial typecheck exposed fixture-only string-indexing and unsupported `exact` role option; both corrected.
- `bun run typecheck`: exit 0 after correction.
- Final test run: `bun run test src/features/model-pricing/__tests__/pricing.test.ts src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx src/features/model-pricing/__tests__/editor-layout.test.tsx`: exit 0, 3 files / 23 tests passed, 7.64 seconds.
- `bunx oxlint -c .oxlintrc.json src/features/model-pricing/api.ts src/features/model-pricing/__tests__/pricing.test.ts src/features/system-settings/models/ratio-settings-card.tsx src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx`: exit 0, no diagnostics.
- `bunx oxfmt -c .oxfmtrc.json --check src/features/model-pricing/api.ts src/features/model-pricing/__tests__/pricing.test.ts src/features/system-settings/models/ratio-settings-card.tsx src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx`: exit 0; all matched files correct. Protected license headers unchanged.
- `git diff --check`: exit 0.

Full web-suite verification is owned by the lead after both workers finish, explicitly coordinated to avoid simultaneous duplicate heavy runs. This report does not claim a full-suite pass.

## Boundaries / concerns

- No backend/auth changes, price values changed in runtime, upstream import/preview/restore requests, container/server actions, commits or pushes.
- No Task 1 files modified by this worker; their shared-worktree changes were preserved.
- Mixed saves intentionally restore canonical protected values on refresh and show dedicated-tab guidance; protected-only edits remain unsaved in the form. The message is existing translated copy, so no locale edits/new strings were needed.
- Dedicated protection here is client-side generic-path safety, not a new server authorization control.
