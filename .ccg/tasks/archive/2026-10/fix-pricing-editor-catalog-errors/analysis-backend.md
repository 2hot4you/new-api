# Independent backend implementation analysis

Analysis only: read requirements/plan, current prospective validation/error/UI paths and prior diagnosis context. No application edits, tests, server calls, pushes or child agents. This is native independent analysis, not external antigravity/Claude verification. Preserve concurrent owners' edits.

## Task2: narrow implementation seam

The coordinator accepted adding **only the necessary typed-error wrapping in `model/model_pricing_config.go`** to Task2 ownership. This is the smallest clean origin: provider identity is known inside `validateModelPricingWithDependencies`, not in its caller `stageCatalogValidation`. Do not duplicate its selection/unchanged/override logic in a separate preflight or parse its formatted error text.

Introduce a model-layer typed prospective-pricing conflict with `Model`, `Provider`, an allowlisted `Reason` and an unexported cause. Initially allow **only** `task_fixed_pricing_unsupported`. Preserve the exact existing `.Error()` string by delegating to the previously constructed error; `Unwrap()` returns that cause. Other pricing callers therefore retain their current message/behavior. Catalog's controller projects selected safe fields, never the cause or `err.Error()`.

Wrap at the exact existing failure branches that call `SmokeTestTaskExpr`: model-level declaring-provider loop, explicit provider-expression validation and resolved-alias provider path where applicable. **After** the original smoke call rejects, classify only if `billingexpr.UsesFixedPricing(expression)` is true. That helper compiles and returns false on compilation failure (`pkg/billingexpr/fixed.go:12-20`), so invalid syntax is not mislabeled as this known reason. Model-level validation already compiles before selecting providers. Retain every original binding, overridden-provider and unchanged-single-provider allowance; do not call the typed path before those branches. No accepted/rejected price or attestation input should change. Generic syntax, undeclared usage, compiler/runtime/DB errors remain untyped/redacted for this bounded fix.

An error type can reside with other catalog validation definitions, but its origin must remain the existing pricing validator. `stageCatalogValidation` should simply propagate it through its existing error return; no new writes, registry reads or semantic acceptance are needed.

## Concrete HTTP and localization shape

Use one fixed non-success status, preferably **422**, separating prospective configuration incompatibility from stale-plan409. Suggested contract:

```json
{
  "success": false,
  "code": "CATALOG_PRICING_CONFLICT",
  "message": "Rendered safe English message",
  "message_key": "Catalog pricing for model {{model}} conflicts with provider {{provider}}. Fixed request pricing cannot be used for task usage billing. Review the source pricing configuration.",
  "message_params": {"model": "grok-imagine-video", "provider": "xai"},
  "data": {
    "model": "grok-imagine-video",
    "provider": "xai",
    "reason": "task_fixed_pricing_unsupported"
  }
}
```

Project with `errors.As` in `catalogSyncError`, while preserving precedence for existing unknown-commit/publication/source/conflict responses. Validate the reason against an explicit allowlist, provider via `jsplugin.ValidPluginKey`, and model as a bounded nonempty trimmed identity without control characters; use conservative printable-identity acceptance or omit unsafe identities/fall back to existing generic error. Do not echo expression text, amounts, SQL, DSN, arbitrary cause text or any fields not required for this contract. Unknown reason, malformed identity, or plain error containing the same text must stay generic503 without details. Keep `catalog_outcome` audit projection to the fixed code, not raw error.

Reuse `common.NewMessage(...).Fields()` plus its rendered English `.Error()` rather than inventing translation machinery. Existing `getServerErrorMessage` already reads `message_key/message_params` from Axios and business-error causes, so catalog's existing destructive Alert can display the safe translated conflict. Do not change global HTTP/error handling or turn this into a successful/selectable plan. All seven locale keys belong to the exclusive locale owner.

On a failed fresh preview, an old unsent `plan/currentPlan/finalPlan` must not leave a seemingly valid Apply action. Invalidate that stale preview state for the new typed failure, but **do not discard** an already-sent operation binding/receipt or alter unknown-commit recovery/retry rules. Existing `retry:false` stays unchanged. Backend failure already occurs before final plan persistence; do not synthesize an executable plan or partially synchronize unrelated prices to obtain success.

## Regression tests: existing files, RED first

1. Model validation: valid compiled `fixed(...)` for `grok-imagine-video`, factory-like xAI metadata/profile, strict/create or strict/adopt input yields typed reason/model/provider and unchanged prior error string/unwrap; task fixed remains rejected. Existing unchanged-single-provider preservation and explicit valid usage override still behave as before. Invalid syntax with fixed-looking text and arbitrary compiler failure are not typed. Extend `model/catalog_sync_test.go`'s staged/preservation tests or existing suitable pricing tests rather than scattering files.
2. HTTP real path: extend `controller/catalog_sync_http_test.go`, using its real PostgreSQL startup/auth/HTTP fixture and a valid source snapshot with new Grok fixed expression plus matching CNY exact metadata; assert422/code/allowlisted details/localization fields, no raw expression/secret marker, no persisted plan/operation/baseline or business-data mutation, state/revision unchanged. The first fenced transaction legitimately updates its coordination anchor, so do **not** assert zero writes to every table or weaken that lock path to satisfy the test. Keep auth/session/proof tests unchanged.
3. Pure projection table in existing controller test: `fmt.Errorf("outer: %w", typed)` recognized; unknown reason, hostile/control-character identity, arbitrary raw error/DB or expression-looking secret text redacted; existing source503/publication503/stale409/unknown commit codes preserved. Use fixed distinctive secret markers and verify they are absent from the whole body.
4. UI existing catalog suite: structured Axios422 and business-failure envelope display localized model/provider/fixed-task reason; unknown503 remains generic; no source/price/metadata mutation, no enabled Apply after failed fresh preview, no auto-retry. Add success-preview→typed-failed-preview stale-state case while retaining durable receipt/unknown-outcome cases.

## Task1 interaction checks

- Keep metadata eligibility distinct from actual save pending; `has_metadata=false` must never be passed as `isSaving`. Unconfigured but exact metadata rows are included in full pricing snapshots (`model/model_pricing_config.go:281-315`), so Add can price an existing exact row without inventing it.
- Whole-batch dirty-name preflight should reject explicit missing metadata **and absent/unknown entries** rather than silently dropping either; filter only the established Seedance-protected identities first. Untouched missing-metadata builtin/default rows must not block unrelated valid edits. A stale snapshot requires explicit reload/single-name preflight, not automatic expected_version refresh of other dirty rows.
- `buildPricingChanges` is shared by generic JSON and upstream sync. All callers must catch a named eligibility error and preserve drafts/selection; no UI should report a filtered partial batch as complete. Test both callers rather than fixing only one card.
- Reset must select only eligible nonempty persisted `configured` overrides, or clearly reject configured blockers; no empty/default-only reset PATCH. Deletion remains a local override change with a persistent form save action after closing the last editor. ExposeRatioEnabled must not save before missing-model preflight rejects the batch.
- `deepseek-fetch` missing metadata and source Grok fixed/xAI conflict are independent. Task1 must not create guessed metadata or change backend currency/version guards, and Task2 must not convert the source quote or relax fixed/task billing.

## Verification / DB matrix boundary

Pure error classification/projection plus frontend state handling changes neither SQL/schema/transaction semantics nor accepted pricing behavior, so the mandatory three-engine **database-behavior** matrix is not newly triggered solely by this bounded patch. Run affected pure model/controller tests, genuine PostgreSQL preview regression and normal/race repository PostgreSQL gates as planned; record zero skips for required selections. If implementation changes transaction boundaries, persistence/rollback, query shape, metadata rules or accepted drafts, that is beyond this boundary and requires the real SQLite/MySQL/PostgreSQL matrix. Never claim cross-engine verification from a PG-only run.

No live source expression, task override, numeric anchor, dedicated Grok/Seedance table, plugin enablement or catalog data needs to change for this error-feedback repair. The deterministic invalid source contract must remain blocked until separately approved pricing ownership is resolved.
