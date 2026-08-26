# Review

## Scope

- Rankings group success-rate rows enriched with configured group icon and description.
- API-key model restrictions display each model's configured model icon.
- API-key IP restrictions use a validated IPv4/IPv6/CIDR chip editor while preserving the existing newline-delimited payload.

## CCG tooling

- Both required external analysis attempts failed because `/Users/naf/.claude/bin/codeagent-wrapper` is unavailable (antigravity exit 127, Claude exit 127).
- Both required external final review attempts failed for the same reason.
- Fallback used two independent read-only analysis agents, scoped task reviewers, and two independent final reviewers with different review focuses.

## Review history

- Task 1 rankings review: approved, no Critical/Important/Minor findings.
- Task 2 model-icon review: approved; one non-blocking duplicate fallback-test gap was deferred and later the persisted-unavailable-model icon path was added and tested.
- Task 3 IP/CIDR initial review found one Critical and three Important lifecycle issues. Two fix rounds closed valid-draft submission, mixed-batch acceptance, controlled-value normalization, legacy invalid blocking, reset behavior, and draft preservation.
- Final dual review found the collapsed Advanced Settings validation bypass and missing locale keys. The single final fix wave kept validation mounted, gated initial submission readiness, synchronized all locale catalogs, canonicalized CIDR networks, and preserved icons for selected unavailable models.
- Scoped final re-review: all findings addressed; no new Critical or Important findings; release-ready.

## Verification

- Frontend focused suites: 47 passed, 0 failed.
- `bun run typecheck`: passed.
- Scoped `oxlint`: passed.
- `bun run i18n:check`: passed.
- `bun run build`: passed.
- `gofmt -l service/rankings.go service/rankings_test.go`: no output.
- `go test ./... -count=1`: passed.
- `go vet ./...`: passed.
- `go build ./...`: passed.
- `git diff --check origin/develop..HEAD`: passed.

## Safety and compatibility

- No database migration or backend token contract change.
- `allow_ips` remains a newline-delimited string.
- No pricing, billing, channels, deployment, main, or production changes.
- Model icons come from model metadata; provider icons are not substituted.
- Missing performance data remains missing; no synthetic success-rate fallback was introduced.

## Rulings

- Preserve configured group order in the rankings service and perform measured success-rate sorting in the frontend. This retains the configured no-data tail order and the existing API behavior.
- Do not add redundant drawer coverage for the shared `getLobeIcon` invalid-key fallback; the final integration instead covers the user-visible persisted selected-model icon path.
