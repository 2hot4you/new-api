# Review

## Root cause

`GroupPricingTable` compared parsed ratio maps by `JSON.stringify` without canonicalizing object keys. After an administrator reordered groups, the display metadata order could differ from the legacy ratio-map insertion order. Typing the first Icon or recommendation character synchronized metadata, the effect misclassified the equivalent maps as changed, rebuilt all rows with new `_id` values, and replaced the focused input node.

## Fix

- Canonicalize the three unordered legacy maps as deterministically sorted entry arrays before comparing signatures.
- Keep ordered `GroupMetadata` as an ordered array so real display-order changes remain observable.
- Add controlled-parent regression coverage for Icon and recommendation inputs, asserting that the original input node remains connected, focused, and updated after metadata synchronization.

## Verification

- Regression test demonstrated RED before the production change: both Icon and recommendation input nodes became disconnected.
- `bun test src/features/system-settings/models/__tests__/group-metadata-order.test.tsx`: 8 pass.
- Related metadata reload and Auto-group validation suites: pass.
- `bun run typecheck`: pass.
- Scoped Oxlint and format checks: pass.
- `git diff --check`: pass.

## Review

- Local diff review found no change to ordering, billing, validation, save, or API Key semantics.
- Required antigravity and Claude reviews were attempted in parallel, but `/Users/naf/.claude/bin/codeagent-wrapper` is not installed; both commands exited with status 127.
- No Critical or Warning issue remains from the available local review.

## Spec evolution

No shared project convention changed; no spec update is needed.
