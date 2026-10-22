# Final whole-branch pricing repair review

## Scope and verdict

Reviewed exact range `915723524..9a58dffae` (seven commits, 32 changed paths), the approved requirements/plan, Tasks 1–3 reports, both scoped reviews, and the actual integrated production/test sources. Read the checkout's root/frontend AGENTS, billing rules and expression specification, and applicable backend/general specs. The final range includes three CCG planning files; application review concentrated on the remaining backend, frontend, locale and test-gate paths.

- **Spec compliance: PASS for the bounded repair.** No actionable deviation from the approved scope found.
- **Code quality: READY for coordinator acceptance, with no code-review blocker found.** This is not a full-test, deployment or promotion approval.
- **Release gate: NOT established by this review.** The coordinator must finish fresh stable-source frontend/build and required normal/race PostgreSQL acceptance, then verify exact-source develop CI/CD before any main promotion. Worker scoped results, the interrupted earlier frontend run, and partial gate progress are not substitutes.

## Findings

- Critical: none.
- Important: none.
- Minor: none requiring a change in this range.

## Integrated contract checks

### Pricing eligibility, versions and drafts

- `web/src/features/model-pricing/api.ts:76` requires explicit exact metadata and an existing USD/CNY source currency. `:224` checks only changed model/currency candidates, collects all metadata blockers, and throws before returning a batch. An unchanged metadata-less builtin does not block an unrelated eligible change. No unknown entry gains an empty version or invented USD authority; eligible writes keep the original snapshot version/currency.
- `web/src/features/system-settings/models/ratio-settings-card.tsx:512` performs the whole-batch preflight before either the pricing PATCH or visibility PUT. The missing-only/mixed JSON regression assertions include retained draft, no PATCH/PUT and no false success. Failure does not advance the baseline. Dedicated Seedance exclusions and routing notice remain.
- `web/src/features/system-settings/models/model-pricing-sheet.tsx:288` checks trimmed Add names against the original baseline entries, adopts known currency, and blocks an unknown name without creating metadata. The imperative draft path at `:824` and Save control at `:1248` both enforce eligibility. The prerequisite Alert at `:908` explains the condition and opens existing metadata management in a separate safe tab, preserving the current draft.
- `web/src/features/model-pricing/model-pricing-panel.tsx:94` independently guards embedded persistence/reset; `:183` passes only real mutation pending as Saving. Desktop/mobile visual-editor call sites likewise keep metadata absence separate from pending. The original full-baseline write authority is not replaced by the editor's separate display query.
- `web/src/features/system-settings/models/model-ratio-visual-editor.tsx:409` treats builtin-only removal as an explanatory no-op; configured removals remain local drafts. Selected-row removal closes the editor, row-menu propagation does not reopen it, and the independent Save action at `:855` survives selected/last-row removal. Failures retain the deletion draft. The label at `model-ratio-table-columns.tsx:200` accurately describes override removal rather than deleting a model/default.
- `ratio-settings-card.tsx:277` limits reset candidates to persisted non-Seedance overrides and rejects all candidates before mutation if any metadata/currency prerequisite fails. Default-only reset is a no-op, and the dialog at `:734` states the scope. No compiled price or metadata is removed.

### Fail-closed catalog conflict and operation recovery

- `model/model_pricing_config.go:86` adds typed identity only after an existing task smoke-test rejection. Classification uses successful fixed-pricing compilation, not arbitrary error-text parsing. The three existing rejection origins remain rejects; formatted error/cause traversal, alias identity, unchanged-single-provider preservation and valid explicit-provider shadowing remain intact. No model-name exemption, price conversion or task `fixed()` acceptance was added.
- `controller/catalog_sync.go:473` projects only the typed allowlisted reason with validated bounded model/provider identities as a sanitized localizable 422. Existing unknown-commit, publication, source and stale-plan precedence remains. `:486` falls back to generic redaction for unsafe/unclassified cases; expression, compiler, SQL, credential and raw cause text are not serialized.
- `model/catalog_sync_pricing_conflict_test.go:16` uses actual xAI factory source and protects create/strict-adopt rejection, preserved unchanged configuration, alias/provider identity, generic-error exclusion and legitimate shadowing. `controller/catalog_sync_http_test.go:227` uses real exact CNY metadata and authenticated disposable PostgreSQL prospective validation; it asserts sanitized 422 and unchanged durable business/plan/revision tables. The projection table at `:274` covers redaction and higher-priority errors. These fixtures do not manufacture acceptance by bypassing metadata or weakening validation.
- `web/src/features/system-settings/catalog-sync/index.tsx:193` clears only failed unsent preview/choices/confirmation state for both sync and restore. It does not clear durable binding, receipt, session storage or original-result recovery. Existing unresolved/publication locks remain (`:325`). The new UI tests exercise HTTP and business failures, unknown failure, and succeeded-binding/receipt preservation; existing uncertain-operation tests are retained.

### New mandatory CI selection and DDL follow-up

- `deploy/tests/test_catalog_postgres_gate.py:412` adds the pure model regression to the nonmatrix model group, ensuring both create/adopt children run rather than being excluded by a `/postgres` selector. `:532` adds both HTTP regressions to the nonmatrix tagged controller group. Existing required names remain; no skips, retries, altered verifier rules, or reduced selection were introduced. The inventory guard at `:910` protects package/group/nonmatrix membership. The same `GROUPS` drive normal and race modes, and the unchanged verifier requires every selected parent plus its existing skip/failure checks.
- `controller/catalog_sync_test.go:850` now runs the actual ALTER inside an explicit cancellable transaction. The test still observes a real PostgreSQL lock wait (`:861`) while the source holds ACCESS SHARE, resumes the source, requires ALTER execution (`:864`), and requires both **no export error and Complete=true** (`:872`, `:877`) before allowing DDL commit (`:878`). It then requires commit success. This fixes the positive fixture's scheduling ambiguity: production's fresh post-COMMIT metadata proof correctly rejects DDL committed before that proof; that proof was not changed.
- The source production path still performs its fresh metadata comparison after releasing read-transaction table locks (`model/catalog_sync_references.go:550`). The test deliberately holds the concurrent catalog change uncommitted until export completion, so it no longer races that intended success assertion against a legitimately detected committed change. No accepted-arbitrary-error, retry or timeout increase replaces the positive assertion.
- Failure cleanup at `controller/catalog_sync_test.go:813` idempotently releases both barriers, cancels both contexts, and joins tracked workers with a bounded failure report before fixture cleanup. The buffered result channels cannot strand workers when an assertion exits early. The later cancellation block at `:890` retains actual lock waiting, cancellation error, writer-barrier availability, blocker rollback and successful fresh export. The changes are confined to this test fixture; production SQL/schema/locking behavior is unchanged.

### Locale preservation and shared UI

- Independently audited the exact committed before/after content of all seven locales: exactly eight added effective keys per locale, every previous effective value unchanged, all new interpolation placeholders preserved. The entire prior raw text prefix remains byte-identical through its final value, followed only by the necessary comma and additions. This preserves duplicate/raw protected attribution references, not merely their decoded semantic equivalent. Example additions begin at `web/src/i18n/locales/en.json:8530` and `web/src/i18n/locales/zh.json:8577`.
- Added messages match the actual shared eligibility helper, editor/reset/removal UI and server message key. The final diff contains no unrelated locale normalization. Existing Alert, Button, ConfirmDialog, row actions and shared pricing editor are reused; no dependency or competing generic interaction implementation was introduced.

## Independently executed checks

- `python3 deploy/tests/test_catalog_postgres_gate.py -v`: **PASS, 6/6**, including new required nonmatrix inventory and existing fail-closed gate tests.
- `git diff --check 915723524..9a58dffae`: **PASS**, exit 0.
- Read-only Node audit of `git show` at both exact revisions: **PASS** for seven locales, eight additions each, original effective values/raw prefix and placeholders preserved.
- `git status --short` was empty before writing this requested ignored report. No application files, index, HEAD, dependency, server or live configuration were changed.

The implementer reports record focused frontend 170/170, catalog UI 27/27, focused/full HTTP and billing suites, corrected PostgreSQL selections, and new DDL normal/race focused passes. I inspected the relevant assertions and source contracts, but did not rerun those broad/application suites and do not present their reported results as my independently observed acceptance. The earlier full frontend exit 130 and original DDL gate failure remain disclosed evidence, not green runs. Coordinator stable full gates were still separate work when this review was prepared.

## Declined to judge / retained release responsibilities

- Final stable-source full frontend tests, complete normal/race PostgreSQL gates, production build/typecheck/lint outputs and exact-source develop release/deployment completion. This review does not authorize promoting unverified intervening commits.
- Cross-engine compatibility beyond the unchanged production database behavior inspected here; no independent SQLite/MySQL verification was run or claimed. The explicit DDL transaction is PostgreSQL test-fixture scheduling, not a production transaction change.
- Live source price ownership, commercial correctness of any Grok quote/currency, live pricing/metadata writes, catalog apply/restore, conversion, plugin disabling, or production database repair. The existing source fixed-Grok/xAI conflict intentionally remains rejected.
- Unchanged auth/session/proof/SSRF architecture, settlement arithmetic, plugin runtime/schema and unrelated repository behavior. No auth contract or database/schema change was found in this range, but this is not a new full security audit.
- Full visual-device/accessibility validation or native-language editorial certification for every translation. Code/regression-level affordances and seven-locale placeholder/preservation contracts were reviewed.
- External antigravity/Claude review: not performed or claimed. This is an independent native-agent review under the documented unavailable-tool substitution.
