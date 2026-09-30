# Review — unified model pricing currency

## Scope reviewed

- Source-currency normalization and frozen rate snapshots.
- Exact-model metadata resolution and atomic pricing/currency writes.
- Expression, Seedance, Grok image, and Grok video settlement.
- Public pricing, quotation, usage-log audit, and administrator editing UI.
- One-time CNY metadata migrations and the audited model allowlist.

The user explicitly prohibited antigravity/Claude external executors, so the required review was performed directly by Codex against the complete `origin/develop...HEAD` diff. No external model reviewer was invoked.

## Findings

### Critical

None found.

### Warning

None introduced by this change.

### Information / repository baseline

- Repository-wide `go test ./... -count=1` remained in package discovery/startup with no package output or child test process for five minutes and was interrupted. Every changed backend package was then run explicitly and passed.
- Repository-wide frontend `pnpm lint` still reports existing lint failures across unrelated files and also scans untracked `* 2` copies. All frontend files changed by this task pass focused oxlint.
- Repository-wide `pnpm format:check` still reports existing unrelated files and untracked `* 2` copies. All frontend files changed by this task were formatted with the repository's oxfmt configuration and disappeared from the failure list.
- The untracked `* 2` files were neither edited nor staged.

## Verification evidence

- `go test ./controller ./model ./pkg/billingexpr ./pkg/billingmoney ./relay ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./relay/common ./relay/helper ./service ./setting/ratio_setting -count=1` — passed all 11 affected packages.
- `go test ./model -run 'TestMigrateModelBillingCurrency' -count=1` — passed.
- `pnpm test` — 290 files and 2726 tests passed in the final formatted state.
- `pnpm typecheck` — passed.
- `pnpm i18n:check` — passed.
- `pnpm build` — production build passed.
- Focused oxlint over every changed frontend TypeScript/TSX file — passed.
- Focused oxfmt over every changed frontend TypeScript/TSX/JSON file — passed.
- `git diff --check` — passed.

## Correctness checks

- USD expressions retain existing USD-equivalent quota semantics.
- CNY expressions normalize source cost by the request's frozen CNY-per-USD rate before group ratio.
- Invalid CNY rates fail closed.
- Mapped requests resolve source currency from the final billing model.
- Async task/Grok completion retains submission-time source currency and rate.
- Legacy ratio/fixed pricing remains USD-based.
- Users see the configured site currency; administrators can additionally inspect source amount, frozen rate, and USD-equivalent amount.
- CNY migration is allowlist-only and versioned; once its marker exists, later administrator edits are not overwritten.

## Deployment acceptance

Pending the first push to `develop`. After CI/CD deploys the change, verify the public DeepSeek pricing display and the deployed configuration before archiving this task.
