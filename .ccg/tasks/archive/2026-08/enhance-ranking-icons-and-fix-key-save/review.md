# Review

## Scope reviewed

- API key create/update drawer close lifecycle and IP/CIDR validation state.
- Rankings API model icon propagation.
- Top Models and Market Share tooltip icon decoration.
- Provider icon rendering in the aggregated vendor ranking.
- New frontend and backend regression tests.

## External model review

CCG-required parallel review was attempted with both `antigravity` and `claude`.
Both commands exited with status 127 because
`/Users/naf/.claude/bin/codeagent-wrapper` is unavailable in this environment.
No external findings were produced.

## Lead review

### Critical

None.

### Warning

None.

### Info

- The API key persistence request already completed successfully; the crash was
  a frontend close-lifecycle update loop. Cleanup now occurs from the controlled
  `open` effect instead of synchronously resetting the still-mounted form inside
  `Sheet.onOpenChange`.
- IP draft callbacks ignore closed drawers and avoid replacing identical state.
  Existing validation and payload formats are unchanged.
- Tooltip markup is generated only from React-rendered `getLobeIcon` output.
  React escapes configured icon keys, and no API response HTML is injected.
- Only known model/vendor rows replace VChart markers. Total, Others, overflow,
  sorting, metrics, and chart colours remain unchanged.
- Missing or invalid icon keys continue through the existing `getLobeIcon`
  fallback.

## Verification

- Frontend affected tests: 58 passed.
- Frontend typecheck: passed.
- Scoped oxlint: passed.
- i18n completeness: passed.
- Frontend production build: passed.
- Frontend format check: passed after formatting task files.
- Repository-wide copyright check: reports pre-existing header drift in 72
  unrelated files; none of the files changed by this task are in that report.
- `go test ./service -count=1`: passed.
- `go test ./controller -count=1`: passed.
- `go test ./... -count=1`: passed.
- `go vet ./...`: passed.
- `go build ./...`: passed.
