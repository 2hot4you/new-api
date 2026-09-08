# Task 3 — catalog, pricing, and channel frontend merge report

## Completed resolution

- Resolved and staged every assigned model, pricing, channel API/type, and
  channel drawer conflict. There are no unmerged entries in the assigned paths.
- Kept Molii administrator-managed marketplace publication, model/vendor
  ordering, capability metadata, mixed-currency catalog, quotation inputs, and
  image/video/Grok/Seedance surfaces.
- Removed the upstream metadata-sync dialog pair and its sync-specific test;
  no sync endpoint or control was restored.
- Added rc.36-compatible model-square/channel visibility data, channel-only
  model listing, deletion/task-pricing UI, and a card/table catalog switch.
- Kept scoped channel-key disclosure: the drawer delegates to
  `useChannelKeyDisclosure`, which obtains a `channel.key.read` proof and
  passes the required proof token to `getChannelKey`.
- Preserved task-plugin metadata and Molii channel model overlays in the
  auto-merged channel API/types.

## Validation

- `git diff --cached --check`: passed.
- Assigned-path conflict-marker scan: passed.
- `bun run typecheck`: not runnable because the worktree lacks `tsgo`
  (`/bin/bash: tsgo: command not found`).
- `bun run test -- features/models features/pricing features/channels`: not
  runnable because the worktree lacks `vitest`
  (`/bin/bash: vitest: command not found`).

## Follow-up

Install the web dependencies (or use the repository's prepared dependency
environment), then run the focused model/pricing/channel test groups and the
web typecheck. The repository-wide merge remains intentionally unresolved in
other owners' paths.

## Fix round 1 — model/pricing contract reconciliation

- Reconciled model deletion with the endpoint contract: callers can now supply
  channel/pricing cleanup flags and receive the structured deletion result.
- Added vendor version support to the form schema and kept vendor status out of
  metadata-edit payloads, so an edit cannot silently reactivate a disabled
  vendor.
- Wired the vendor section through the models context and dialog registry,
  including create/update vendor state and the Vendors tab category.
- Kept retired metadata synchronization removed by deleting the stale
  MissingModels dialog action that opened the removed sync wizard.
- Made catalog view controls backward-compatible for existing callers while
  retaining the card/table contract, and added the task-price conversion
  options consumed by dynamic pricing breakdowns.

### Validation

- `bun run typecheck`: passed (`tsgo -b`).
- `bun run test -- src/features/models/__tests__/vendor-management.test.tsx src/features/pricing/components/__tests__/model-directory-controls.test.tsx`:
  passed (2 files, 9 tests).

### Remaining focused-suite observations

- `task-price-display.test.tsx` still has six expectation failures around
  labels, rule rendering, and legacy compact currency formatting; the prop
  contract/typecheck failure is resolved.
- Legacy model-listing tests still expect the intentionally removed upstream
  synchronization policy UI, and metadata-editing assertions see the retained
  Molii marketplace publication notice. These expectations conflict with the
  merge requirements rather than this repair.

## Fix round 2 — catalog pricing and model-management suite

- Hardened dynamic tier presentation against missing labels and retained the
  local, human-readable task condition labels in desktop and mobile pricing
  tables.
- Normalized task and token price presentation to compact, trimmed values
  (for example `$5/1M token`) while preserving mixed currencies, custom usage
  schemas, quotation inputs, Seedance, Grok, and GPT Image 2 pricing.
- Restored catalog card pagination and accessible compact pricing/card metadata
  without reintroducing upstream synchronization or embedded pricing editing.
- Reconciled vendor editing with local metadata-only update semantics and made
  its icon selector accessible by name.
- Updated stale tests that asserted retired sync-policy UI or former expanded
  card pricing, and made metadata-save tests wait for the actual mutation and
  selected vendor state rather than timing-sensitive intermediate DOM.

### Validation

- `bun run test -- features/models features/pricing features/channels`:
  passed (56 files, 365 tests).
- `bun run typecheck`: passed (`tsgo -b`).
- `bun x oxfmt --check` over all 14 changed owned files: passed.
- `bun x oxlint -c .oxlintrc.json` over all 14 changed owned files: passed.
- Repository-wide `bun run format:check` and `bun run lint` still report
  pre-existing failures in many unowned files; no unrelated files were
  changed to mask those baseline failures.

## Follow-up — full-suite metadata stability

- Replaced per-character user-event interactions in the pricing-conflict
  recovery test with synchronous input events. The test still verifies that
  metadata and price drafts survive tab changes, that a stale version requires
  reload, and that the refreshed version is saved independently.
- This removes full-suite CPU-contention timing while keeping the default test
  timeout unchanged.

### Validation

- `bun run test -- features/models features/pricing features/channels`:
  passed (56 files, 365 tests; target test 1.7s in its focused run).
