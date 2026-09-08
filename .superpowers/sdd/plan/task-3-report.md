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
