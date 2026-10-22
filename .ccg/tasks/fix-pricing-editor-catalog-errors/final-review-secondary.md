# Secondary independent whole-branch review

## Verdict

- **Spec: PASS for the reviewed bounded repair and explicitly approved follow-ups.**
- **Quality: PASS; no actionable Critical, Important, or Minor defect found.**
- **Release gate: NOT JUDGED / still pending coordinator verification.** This code review does not turn partial or earlier test results into final-source integration green.

Reviewed `915723524..9a58dffae` (seven commits, 32 changed paths) and the supplied `review-915723524..9a58dffae.diff`, requirements/plan, Task 2/3 reports and Task 2 review. My earlier `task-1-review.md` remains applicable: independently confirmed there are no subsequent changes to its 14 reviewed application/test paths. I did not repeat that wholesale inspection; this pass concentrates on backend/error identity, frontend failure-state interaction, final CI inventory/DDL fixture and cross-task locale integration.

Applicable root/frontend AGENTS, backend/general specs, billing rules, expression-system contract and shadcn composition constraints were consulted. No tests were rerun. No application, git/index/HEAD, deployment, or concurrent reviewer state was changed. Only this requested report is written.

## Findings

- Critical: none.
- Important: none.
- Minor: none.

There are therefore no severity/file:line defects requiring a remedy in this range. The following are review evidence and explicit boundaries, not findings.

## Backend/error and frontend integration evidence

1. **The original fixed/task rejection is preserved.** `model/model_pricing_config.go:86` classifies only after the existing `SmokeTestTaskExpr` rejection at explicit-provider/model-provider/alias origins (`:489`, `:537`, `:553`). `setting/billing_setting/tiered_billing.go:302` still compiles first, rejects successfully compiled `fixed()` task expressions before usage validation/evaluation, and retains all other schema/vector rules. `pkg/billingexpr/fixed.go:12` classifies from the compiled AST flag, not a model-name exception or arbitrary error text. The wrapper's `Error`/`Unwrap` at `model/model_pricing_config.go:77` preserve prior text and cause; unchanged-single-provider/stale-override preservation and valid explicit-provider shadowing remain untouched. Neither quote values nor expression execution/acceptance rules change.

2. **The typed cause reaches catalog HTTP without changing staging/persistence.** The existing staging validator directly propagates the typed error (`model/catalog_sync_validate.go:256`); the existing caller returns before attestation-derived plan storage (`model/catalog_sync_apply.go:195`). Only model/provider/reason identity is added. The new real xAI regression at `model/catalog_sync_pricing_conflict_test.go:17` covers create/adopt rejection, alias/provider identity, preserved error text/cause, unchanged-expression allowance, unrelated syntax/schema/runtime errors and valid provider shadowing. The HTTP regression at `controller/catalog_sync_http_test.go:227` uses real exact CNY metadata plus a structurally valid manifest and asserts failed preview leaves business/revision/plan/operation/baseline state unchanged. It does not relax the startup/metadata contract or apply a live quote.

3. **Projection is narrow and redacted.** `controller/catalog_sync.go:473` uses `errors.As` only after existing uncertain-commit, stale, unavailable-source and publication-pending handling. `:486` requires the single allowlisted reason, canonical <=30-byte plugin key and conservative nonempty <=255-byte ASCII model identity. The 422 response is constructed from a fixed template and these identities; it does not serialize the cause, expression, SQL or credentials. Unknown/unsafe errors retain generic 503. The projection table at `controller/catalog_sync_http_test.go:269` explicitly covers raw-text imitation, unsafe model/provider, unknown reason and higher-priority error precedence. No authentication, proof, session, SSRF or backend version/currency fence is edited.

4. **HTTP and business failures produce the same safe localized feedback.** The response's `message_key`/`message_params` follow the existing `common.Message` and frontend `getServerErrorMessage`/`requireServerSuccess` contract. `web/src/features/system-settings/catalog-sync/index.tsx:193` clears only unsent plan/current/final selection and confirmation state for both preview handlers. This removes stale executable UI after rejection while retaining binding, receipt, session storage and original-operation recovery. Existing busy/mutationLocked checks (`:315`) prevent competing preview/apply while an operation is unresolved/publication-pending; no replay/retry was added. The existing semantic destructive Alert (`:461`) displays the sanitized message. Added tests cover HTTP422, success:false, unknown preview failure and preservation of a durable succeeded binding/receipt; existing proof-refusal/uncertain-outcome tests are retained. A separate failed-restore interaction is not newly tested, but both mutation handlers use the same inspected callback.

## CI inventory and DDL fixture follow-ups

- `deploy/tests/test_catalog_postgres_gate.py:412` adds the pure `TestCatalogSyncGrokFixedPricingConflict` to the existing nonmatrix model group, so its create/adopt children are not filtered by `/postgres`. Lines 532–533 add both real HTTP/projection regressions to the existing nonmatrix controller group. The inventory unit guard at line 910 checks package, nonmatrix selection and required-name inclusion. Existing required cases, selector/skip verification, inventory validation, timeout policy and runner behavior are not removed or weakened.
- `controller/catalog_sync_test.go:804` now expresses a deterministic positive DDL schedule rather than allowing autocommit to race the fresh final metadata proof. A real cancellable transaction executes the real ALTER, the original PostgreSQL lock-wait observation remains at line 861, and `ddlApplied` establishes execution before the assertion of a complete successful export. Commit is held until successful export/final-proof completion at lines 871–878. Production's final fresh metadata query and fail-closed comparison in `model/catalog_sync_references.go:548` are unchanged. The fixture neither accepts “metadata changed” as success nor retries/skips the proof.
- Failure cleanup at `controller/catalog_sync_test.go:813` releases both once-only barriers, cancels source/DDL contexts and joins registered workers with a bounded failure report. Buffered result channels do not strand a worker when a fatal assertion unwinds. The existing cancellation half still observes the actual table-lock wait, cancels the exporter, requires failure, proves writer-barrier release and verifies a fresh export after rollback (`:898`–`:909`). The complete flag is delivered before the error on buffered channels, so receiving successful completion before reading it is ordered, not a timing assumption.
- Reported follow-up evidence is normal/race targeted three-source-test success, six inventory unit checks, four existing CI checks and actual updated-selector verification for all three new required regressions. Those are implementation-report results, not tests executed by this reviewer and not a substitute for the coordinator's final whole normal/race acceptance.

## Locale/cross-task integrity

Independently performed a read-only comparison of the two commit trees for all seven locale files. For each, it confirmed exactly eight added effective keys, no prior effective value deletion/change, identical placeholder sets, and every original raw line retained in order byte-for-byte except the syntactically required comma on the original final entry. Thus both protected raw attribution references/escapes survive; semantic JSON equality alone was not used as the raw-reference proof.

The eight additions match Task 1's seven source keys plus Task 2's catalog conflict template. `{{models}}` remains only the named pricing-blocker list; `{{model}}`/`{{provider}}` remain the server's sanitized identities. Translations preserve rejection, draft retention, builtin/default non-deletion, dedicated Seedance exclusion and all-or-nothing reset meaning; no string claims the incompatible quote is automatically repaired. The final net locale diff is eight additions per language, not prior normalization/sorting churn. No permanent locale-script behavior or unrelated reference is changed.

The Task 3 report explicitly explains why its last sync is before protected raw restoration: rerunning that normalizer afterward would remove a protected literal alias. The final baseline-preserving diff and static integrity checks substantiate this narrow exception; it is not a claim that all pre-existing untranslated/missing source keys were fixed.

## Declined to judge / release boundaries

- The coordinator's currently running final-source full frontend, normal/race PostgreSQL, build/type/lint and release gates. Earlier interrupted frontend runs, the Task 1 scoped 170-test green and targeted follow-up green do not establish that final integration is green.
- Broad SQLite/MySQL acceptance, existing unrelated architecture/authentication/settlement/SSRF behavior, or a full independent database security audit. Production SQL/schema/transaction semantics are not changed here; the DDL change is confined to a disposable test fixture.
- Commercial correctness of the existing Grok/source quote or any desired quote ownership/conversion. The incompatible source remains rejected; this patch performs no automatic price/currency/metadata creation, conversion, deletion, plugin disabling or catalog apply/restore.
- Complete real-device visual/keyboard/screen-reader audit, native-level linguistic review of every locale, or unrelated existing untranslated keys. This review checks the changed semantics, component/error contracts and static locale integrity.
- Live deployment/source health, exact-SHA CI/CD status, branch promotion/push/merge or production effects. No live mutation or release action was performed or authorized by this report.
