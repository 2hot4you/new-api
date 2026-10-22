# Task 2 independent spec and quality review

Reviewed range: `dea3caa5a..81f770e40` (head `81f770e40f7c4d53a404c1b3ee2b7c7cfdbaf671`), exactly the six application paths listed in the supplied review package. The three production files inspected in the worktree had no differences from that head. Concurrent Task 1 and locale work was not modified or evaluated.

## Verdict

- Spec compliance: PASS for the bounded Task 2 implementation.
- Code quality: PASS; no concrete Critical, Important, or Minor defect found in the reviewed range.
- This is a scoped code review, not release approval or an independently executed full acceptance run.

## Findings

- Critical: none.
- Important: none.
- Minor: none.

## Contract evidence

1. **Classification originates at real rejection, not error text.** `model/model_pricing_config.go:86` is reached only after `SmokeTestTaskExpr` fails at the explicit-provider, model-provider, or resolved-alias origins (`:489`, `:537`, `:554`). The existing smoke helper compiles first, then rejects fixed pricing before schema/vector execution (`setting/billing_setting/tiered_billing.go:302`). `UsesFixedPricing` returns true only for a successful cached compile with the fixed-pricing AST flag (`pkg/billingexpr/fixed.go:12`). Consequently malformed fixed-looking syntax and unrelated schema/runtime failures are not reclassified. There is no Grok name exception or string parsing of an arbitrary error.

2. **Ordinary validation semantics remain intact.** `model/model_pricing_config.go:77` preserves the old formatted error text, and `:84` retains the original formatted error as its cause, so existing `errors.Is`/`errors.As` traversal remains possible. The new wrapper adds an error-chain layer; it does not replace the cause. The alias branch returns the same prior model-prefixed text early. Unchanged-single-provider preservation, stale overrides, provider shadowing, compilation, metadata/currency checks and all accept/reject decisions remain untouched. The ordinary pricing controller still bases status on its existing sentinel and displays `err.Error()` (`controller/model_pricing_config.go:67`).

3. **Catalog staging retains identity without changing persistence.** The captured effective metadata becomes `jsplugin.Meta{Key: key, ...}` before invoking the existing validator, and the error is returned directly from staging (`model/catalog_sync_validate.go:256`). `model/catalog_sync_apply.go:195` stops before plan digest/storage on staging failure. Neither staging, SQL, migrations, runtime publication nor metadata capture was edited. The real xAI factory source is exercised by the new model test, including strict/adopt versus unchanged preservation, alias identity and explicit-provider override behavior. The HTTP test uses valid exact model metadata, rather than bypassing the metadata requirement.

4. **HTTP projection is narrow and redacted.** `controller/catalog_sync.go:473` accepts only the typed conflict after existing unknown-commit, stale-plan, source-unavailable and publication-pending cases. `:486` allows exactly `task_fixed_pricing_unsupported`, a canonical bounded plugin key, and a nonempty <=255-byte conservative ASCII model identity. The response is constructed from a fixed localizable template plus these identity fields; neither cause nor expression is serialized. Unclassified and unsafe values retain the prior generic 503 response. The new projection table checks raw-error redaction, unsafe identities, unknown reasons and higher-priority error precedence.

5. **Failed replacement previews cannot leave an executable stale preview.** `web/src/features/system-settings/catalog-sync/index.tsx:193` clears the original/current/final unsent plan, choices and confirmation dialog for both preview mutation handlers (`:205`, `:211`). It does not touch binding, receipt, session storage or receipt query state. Existing `busy` and `mutationLocked` checks prevent competing preview/apply actions while an operation is unresolved or publication is pending; original-result recovery remains outside the plan-rendering branch. No automatic retry, source mutation, conversion, removal or plugin change was introduced. The existing destructive Alert and shared error-message translator are reused rather than introducing a competing UI mechanism.

## Verification evidence and boundaries

I read the task brief, plan, complete report and supplied diff, plus the applicable root/web AGENTS, `.agents/rules/billing.md`, `pkg/billingexpr/expr.md`, and available backend/general specs. Cross-diff reads were limited to classifier semantics, catalog propagation, the ordinary caller and operation-state contracts named in the assignment.

Per instruction, I did not rerun broad tests. The implementation report supplies these exact relevant results:

- Focused model Grok regression PASS, including real source/create/adopt, unchanged preservation, alias/provider identity, unchanged error text/cause, unrelated-error exclusion and shadowing.
- Selected PostgreSQL model regression children PASS (`15.015s`), with the corrected `/postgres$` selection explicitly disclosed.
- Real PostgreSQL HTTP Grok/projection tests PASS (`5.196s`); failure leaves models/vendors/options/plans/operations/baselines/catalog state unchanged. The coordination anchor is explicitly outside that assertion.
- Full catalog HTTP suite PASS (`121.235s`), retaining security, unknown-commit and publication tests.
- Full billing-expression and billing-setting package suites PASS.
- Catalog frontend test file PASS (27/27), including HTTP/business failure invalidation, unknown preview error, existing uncertain-operation recovery and new succeeded-binding preservation; typecheck, scoped lint, formatting and diff checks PASS.

Remaining evidence boundaries, not defects:

- The new failed-preview tests drive sync preview, not a separate failed restore-preview interaction. Restore uses the same reviewed callback and its existing successful restore contract remains in the test file.
- Non-ASCII/unusual model identities intentionally fall back to generic 503. This is the conservative projection policy documented in the report, not a claim that every valid model name receives detailed feedback.
- Locale completeness, whole-frontend build/tests and normal/race integration acceptance belong to Task 3/coordinator. This review does not certify those outputs.
- No SQLite/MySQL run is claimed. This patch changes typed errors and UI state only, with no SQL/schema/transaction or acceptance decision change found; it therefore does not independently introduce a new database-behavior matrix requirement.

## Declined to judge

- Task 1 editor/save changes and Task 3 locale changes outside this six-path range.
- Existing unrelated authentication/session/SSRF architecture, billing settlement calculations, runtime/plugin schema behavior and catalog transaction design; none were changed by Task 2.
- Correctness or commercial desirability of the live source quote/currency, automatic conversion, plugin disabling, deleting configured prices, applying/restoring a live catalog, or permitting `fixed()` in task usage.
- Production deployment, CI/CD status, live health, and any live write. No application edits, index/HEAD changes, pushes, subprocess reviewers or production operations were performed for this review. Only this requested report was written.
