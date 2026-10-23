# Task 1 independent backend review

Reviewer: native independent task reviewer, transparently substituting for the unavailable external wrapper. This is not an antigravity or Claude review, nor the later integrated/full-gate review.

## Scope and evidence

Reviewed the immutable `task-1-review.diff` once against `a8e31cc44`, together with the brief, implementation report, approved requirements and plan Global Constraints. The reviewed patch is the three owned files: `model/model_pricing_config.go`, `model/model_pricing_removal_test.go`, and `deploy/tests/test_catalog_postgres_gate.py`. It is an uncommitted worktree patch; no assumption that HEAD contains it.

Read unchanged surrounding code only to resolve concrete risks: read/write currency projection, exact identity/provider leaf replacement, captured input/retry and authoritative transaction recheck, prospective runtime staging/publication, existing fresh-preparation coverage, controller request decoding and root route authorization. No product, index, branch or unrelated-file changes were made.

## Spec Compliance: PASS

- `model/model_pricing_config.go:595-609`: remove-only input rejects blank identities and any Reset, nonempty Pricing or nonblank currency intent. Duplicate identity and required expected-version checks remain. The Seedance guard still runs first at lines 592-594.
- `model/model_pricing_config.go:614-624`: CAS uses complete configured pricing plus the captured normalized exact currency. Only absent metadata gets USD for version comparison, matching `LoadModelBillingCurrencies`/`GetModelPricingSnapshot`; no fallback becomes metadata or currency write authority.
- `model/model_pricing_config.go:628-635`: only option maps present in `draft.previous.options` are passed to `replaceModelPricing`. Shared inner maps deliberately update the detached candidate, while absent maps remain unchanged. The existing helper deletes all valid flattened provider identities for the exact model and does not insert any leaves when its draft is nil.
- `model/model_pricing_config.go:637-660`: ordinary save and Reset retain exact-metadata prerequisites, version comparison, validation and existing currency handling. There are no target-model special cases or changes to root authorization.
- The branch still returns through `savePreparedModelPricing`: preparation errors abandon the entire detached batch; unchanged raw rows/absence are omitted from updates; prospective staging, final whole-input recheck, SQL transaction, postimage verification and runtime acknowledgement remain unchanged. Remove does not populate `draft.currencies`.
- `deploy/tests/test_catalog_postgres_gate.py:428`: the exact new matrix parent is added to `model-runtime-pg`; existing tests and `matrix: True` are retained.

## Quality: PASS

### Critical findings

None.

### Important findings

None.

### Minor findings

None requiring a change for this bounded task.

### Strengths

- Narrow implementation reuses existing exact-model/provider replacement and the existing staging/commit path, avoiding a second mutation mechanism.
- The new tests invoke the public JSON removal intent and assert persisted options, complete metadata rows, version projection, runtime cache content and durable revision/publication state rather than testing copied helper logic.
- The matrix checks invalid intent, ordinary orphan/Reset rejection, stale remove, remove followed by stale/invalid/undeclared-provider/ordinary-orphan members, successful multi-orphan cleanup, exact CNY preservation, builtin fallback, untouched raw text and absent/default-map no-ops.
- The absent legacy default assertion (`gpt-4` ModelRatio = 15) is particularly valuable: it exercises a nonempty read version for an absent persisted map, proving removal neither compares only persisted leaves nor persists a default-suppressing empty map.

## Verification and limitations

Reviewer independently ran:

```sh
env GOWORK=off CATALOG_SYNC_POSTGRES_ONLY='' go test ./model -run '^TestModelPricingRemovalDatabaseMatrix$/^sqlite$' -count=1 -timeout=120s -v
```

PASS, exit 0, package 1.359s; SQLite 3.50.4, all six scenario children passed, zero skips. This used the real existing disposable fixture and changed no product files.

The worker report records relevant pre-implementation PostgreSQL RED and focused PostgreSQL GREEN, then all SQLite/MySQL/PostgreSQL children GREEN, plus gate inventory unit checks. Those reported non-SQLite results were inspected but not independently rerun here, in accordance with coordinator ownership of broad/full acceptance.

The newly added matrix does not independently inject a concurrent writer between remove preparation and commit. Confidence in that boundary comes from the unchanged common authoritative-recheck/retry implementation and inspected existing `TestCatalogRuntimeModelPricingFreshPreparation`; integrated/full gates must still establish their current passing status. Likewise, route authorization was inspected, not newly exercised through HTTP by this task review.

Conclusion: task-level backend patch is approved with no requested changes. Full gates and the independent integrated review remain required before overall completion.
