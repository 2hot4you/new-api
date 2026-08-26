# Review report

## Outcome

Approved after task-level and final backend/frontend review. No open Critical or Warning findings.

## Review history

- Backend group-metadata cleanup: replaced direct JSON encoding with project `common.Marshal`/`common.Unmarshal` wrappers; re-review approved.
- Performance aggregation: added exact 8760-hour boundary coverage and project cross-database group-column convention; re-review approved.
- Rankings integration: moved group summaries into the existing public rankings snapshot to preserve anonymous access; metrics failures only mark the group section unavailable; behavior-level integration coverage replaced a source-inspection test; re-review approved.
- API keys: removed the stale `recommendation` field from the shared `getUserGroups` response type; re-review approved.
- Group pricing: prevented malformed metadata from being destructively normalized and changed tests to verify actual editor emission; re-review approved.
- Final frontend review found that the visual editor could overwrite malformed metadata during an unrelated edit. Commit `1c256770a` now guards every metadata-emitting path until the source is repaired, with mounted regression coverage; final re-review approved.

## Verification

- `go test ./setting/ratio_setting ./model ./pkg/perf_metrics ./controller -count=1`: pass.
- `go test ./... -count=1`: pass.
- `go vet ./...`: pass.
- `go build ./...`: pass.
- Rankings tests: 3 pass.
- API-key group tests: 16 pass.
- Group-metadata defaults test: 1 pass.
- Group-pricing editor tests: 8 pass.
- `bun run typecheck`: pass.
- Scoped `oxlint`: pass.
- `bun run i18n:check`: pass.
- `bun run build`: pass.
- `git diff --check d5ca41eb..HEAD`: pass.

The repository-required Antigravity and Claude wrapper commands were attempted in parallel, but `~/.claude/bin/codeagent-wrapper` is not installed in this environment. Independent local backend and frontend CCG reviewers were used instead; both final verdicts are approved.

## Scope and safety

- No billing ratios, API-key routing order, retry behavior, deployment configuration, production environment, or secrets were changed.
- Recommendation metadata is removed from canonical persistence and public/frontend types without a database schema migration because it lived inside the existing option JSON.
- Real `0%` remains measured data; only zero-request groups display no data.
