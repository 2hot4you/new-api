# Independent final integrated review — secondary

## Scope and verdict

Native independent review substituting for the unavailable external wrapper; no external-model review is claimed. Reviewed the frozen six-path `final-review.diff` against `a8e31cc44`, Task 1 brief/report, approved requirements and Global Constraints. Prior Task 2 frontend review stands; this pass focused on the backend contract and its concrete surrounding storage, currency, staging, transaction, publication and required-test-selector code.

Spec Compliance: PASS for the reviewed integrated patch.

Quality: PASS for the reviewed integrated patch, pending coordinator verification acceptance.

Critical: none. Important: none. Minor: none.

## Concrete evidence and strengths

- `model/model_pricing_config.go:35` and `web/src/features/model-pricing/api.ts:263`: the frontend's literal `remove: true` payload is consumed by the backend JSON field; empty pricing and original expected-version are preserved, with no currency supplied by the frontend.
- `model/model_pricing_config.go:592`: the dedicated Seedance preguard is retained ahead of mutation. Removal rejects blank identities, conflicting Reset, nonempty pricing and nonblank currency; duplicate identity and absent expected-version guards still apply to every batch member.
- `model/model_pricing_config.go:618`: only absent-metadata version comparison substitutes USD, matching `LoadModelBillingCurrencies`' read projection (`model/model_billing_money.go:50`). It does not create a metadata record or become currency write authority. Exact metadata retains its actual normalized currency in the version comparison.
- `model/model_pricing_config.go:628`: the removal view includes only pricing map options present in captured persisted `previous.options`. It references detached draft maps, so `replaceModelPricing` removes exact model and parsed flattened provider leaves without touching absent/inherited maps. Its nil draft cannot insert replacement values. `savePreparedModelPricing`'s unchanged-map comparison preserves both absent rows and untouched raw option strings.
- `model/model_pricing_config.go:635`: removal exits its branch before the ordinary currency-write queue at line 660. Ordinary add/edit/empty-save and Reset continue through exact-metadata, version and existing value/provider validators; there is no general validation bypass for new or changed expressions.
- `model/model_pricing_config.go:611`: all members still transform one detached draft. Any invalid/stale member rejects preparation before SQL/publication. The existing `stageProspectiveCatalogRuntime` path revalidates the complete candidate, then `commitOrdinaryCatalogMutationGuarded` rechecks authoritative input under the final transaction and writer before SQL. Runtime publication/acknowledgement remains after commit; no alternate publication or recovery path was added.
- `router/api-router.go:221` and `controller/model_pricing_config.go:59`: the existing root-authorized PATCH route and shared request consumer remain intact; no new endpoint or permission bypass.
- `model/model_pricing_removal_test.go:26`: the matrix uses actual existing SQLite/MySQL/PostgreSQL fixtures, real actor/enrollment/recovery prerequisites and the public JSON intent. Assertions cover actual rows, unchanged metadata/currency, exact raw unrelated values, absence of inherited maps, version/digest/revision/runtime acknowledgement, stale and invalid mixed batches, ordinary orphan/Reset rejection and compiled fallback. These are not copied implementation helpers standing in for persistence.
- `deploy/tests/test_catalog_postgres_gate.py:428`: the exact new matrix parent is added without removing prior selections. `matrix=True`, the PostgreSQL child selector and verifier's parent/child success and no-skip checks remain intact.
- Frontend strict eligibility, configured-versus-display handling, literal payload shape, atomic preflight and real remove/save success/failure behavior are covered in `task-2-review.md`; no new frontend concern arose from the backend contract.

## Verification evidence and declined-to-judge items

- Task 1 report records a relevant behavioral RED before production changes, then focused normal success across SQLite 3.50.4, MySQL 8.4.11 and PostgreSQL 15.18, all 18 scenario children and no skips; it also records PostgreSQL-only rerun and selector-unit success. Task 2 report records frozen-source focused 44/44 plus typecheck/scoped lint/format success. These are worker-reported, not independently rerun by this reviewer.
- Coordinator reports the focused three-engine race matrix now passes. Full frontend/normal/race gates are still coordinator-owned/in progress; this review does not certify their result or declare the overall task complete.
- Declined to judge live production mutation/deployment, production data cleanup, original browser drafts, unreviewed unrelated dirty files, and operational rollout readiness; these are outside the bounded immutable patch and no such actions were performed here.
- Did not rerun broad gates, spawn agents, edit application/test files, alter index/HEAD/branch, commit or push. Only this review document was written.
