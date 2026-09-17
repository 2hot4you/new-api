# Review: Public Service Group Visibility

## Outcome

No critical or warning-level issues were found in the final local review.

The change separates public service-group discovery from the legacy routable-group fallback. Public pricing and rankings no longer expose identity-only groups, while authentication, token routing, and the `identity -> service group` special ratio lookup keep their existing behavior.

## Regression coverage

- Identity-only `zhoujian` is excluded from selectable public groups.
- Explicit special group additions and removals remain effective.
- Legacy `GetUserUsableGroups` still includes the current identity group for compatibility.
- `zhoujian -> ByteDance = 0.77` remains in the pricing response without exposing `zhoujian` as a service group.
- Pricing groups without a visible model are omitted.
- Rankings exclude active ratio groups that are not marked user-selectable.

## Verification

- `go test ./service ./controller`: passed.
- Focused pricing/rankings Vitest suite: 3 files, 8 tests passed.
- `bun run typecheck`: passed.
- Affected-file `oxlint`: passed.
- `bun run build`: passed.
- `go test ./...`: passed.
- `bun run test`: 215 files and 1502 tests passed; one unrelated existing suite failed because `src/lib/__tests__/site-brand.test.ts` imports `node:test`, which Vitest cannot bundle.
- `git diff --check`: passed.

## External review limitation

The required antigravity and Claude review commands were both attempted in parallel. Both exited with status 127 because `/Users/naf/.claude/bin/codeagent-wrapper` does not exist on this host. No external-model review is claimed.

## Scope check

- No billing formulas were changed.
- No database schema or migration was changed.
- No authentication or token-routing call site was switched away from the legacy resolver.
- No unrelated files were modified.
