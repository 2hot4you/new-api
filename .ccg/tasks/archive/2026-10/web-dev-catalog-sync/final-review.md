# Whole-feature independent review — managed dev catalog synchronization

Range: `64fb668a2778fd848d44e44a263e1a070f48a199` → `0dbc5053b4ecb357d46b69c04c78c756a0c9fb20`.

Reviewed against `.ccg/tasks/web-dev-catalog-sync/design.md`, the approved implementation plan and its PostgreSQL-only override, and the final-review brief. This is an independent integration review, not an inference from earlier task acceptance. No tests, database/container commands, live services, external executors, subagents, regenerated diffs, or implementation changes were used. This report is the only file written.

## Coverage and evidence

The supplied extended-context diff is approximately 1.45 MB. I reviewed it in bounded, risk-directed passes and read the current changed implementation for the corresponding control flows. I do **not** claim every line of every test, locale, setting table or platform branch was read.

- Source/security: complete fixed transport and environment/reader configuration; management/source guards and HTTP orchestration; the new proof context branches; snapshot projection, manifest coverage/digest checks, managed three-way diff and resolution; PostgreSQL source read/metadata proof.
- Mutation/recovery: plan preparation and final rechecks; apply transaction, immutable replay binding, backups and publication; restore preimage/incarnation/baseline logic; reference capture/decision and PostgreSQL transaction/namespace checks; runtime staging, ordinary-writer coordination, read gating, startup eligibility. Read the changes to startup, ordinary options/pricing/metadata/passkey/sort paths, channel cache and pricing projection. Some long diff outputs were truncated, so conclusions here are limited to the named current-source paths and visible hunks, not an assertion of complete diff coverage.
- Consumers: request pricing capture and immutable selection; helper and image/task orchestration diff; Grok/Seedance adaptor selection, Grok snapshot, tool surcharge and audio/realtime consumer changes. Sampled the image/task characterization tests and actual-submit evidence. Did not audit every unrelated billing consumer.
- UI: complete catalog API, index lifecycle, changelog, confirmation and history components; focused tests for StrictMode conflict controls, repeated resolve, unknown/pending receipt and restore; keyboard test boundary.
- Verification/configuration: final Task8 report and independent report; actual final normal/race wrapper summaries and full-web summary; representative upgrade fixture and restricted-role load/rollback fixture; CI/deploy integration and development/molii environment examples. Other environment examples and complete individual raw event streams rely on the supplied Task8 evidence, not a fresh exhaustive review.

Focused unchanged-code checks, each for a named integration risk:

1. `middleware/secure_verification.go:39`: whether a proof rejection can create a catalog operation. It returns the shared proof error before the controller calls the mutation engine. This establishes I1.
2. `model/model_billing_money.go:18`: whether the new capture actually freezes currency/FX and preserves the existing missing-metadata read semantics. It resolves currency from persisted exact metadata and copies the money context. This was not a whole money-engine audit.
3. `middleware/rate-limit.go:169`: whether route registration inherits a preview limiter. It inherits the existing configurable global API limiter; explicitly disabling global limits still disables that limiter. Dedicated export reader limiting is independently present in the new service.
4. `service/task_billing.go`, focused symbol locations for saved ratios and terminal recalculation: to distinguish retained task settlement arithmetic from the new capture change. No broader task subsystem crawl or execution was performed.

The preserved wrappers finish with 824 passing tests/subtests across 27 exact selections for both normal and race gates. The web log finishes with 291 files / 2,748 passing tests and also contains jsdom diagnostics. The Task8 report supplies the remaining build/type/lint/i18n/deploy evidence and exact raw-log locations. None of these checks was rerun by this reviewer.

## Strengths

- The transport fixes the URL, TLS server name and public-IP dial destination, rejects redirects and mixed private/public DNS answers, and enforces both encoded and expanded bounds with the 15-second request lifetime (`internal/catalogtransport/transport.go`). Dedicated reader credentials cannot obtain dashboard identity or mutation privileges.
- Authorization is layered appropriately: exact configured browser origin, interactive current root session, final server-owned plan/digest, then a one-use proof bound to target/kind/operation, and current authentication rechecked inside mutation transactions (`controller/catalog_sync.go`, `model/catalog_sync_apply.go`). Exact committed replay does not burn another proof.
- Managed identity is leaf-level, includes target incarnations, and preserves target-only leaves. Model/currency/price conflicts share an opaque confirmation unit. Deletion checks are conservative, especially unproved price fallback and unfinished references; the feature does not silently weaken those checks to make deletion easier.
- Compilers run on detached inputs outside SQL fences and runtime locks, followed by authoritative input rechecks. Ordinary writers use the same publication path. Durable `committed_pending_publish`, immutable receipts, runtime acknowledgement and startup recovery distinguish a committed write from successfully published pricing.
- Restore uses local saved preimages and verifies affected current postimages and incarnations; it does not fetch dev or overwrite unrelated current objects. The raw backups and authentication bindings are omitted from HTTP projections.
- Existing billing evaluators consume captured values rather than being replaced with another pricing engine. The final evidence includes real controller persistence/reload/terminal consumption with a controlled upstream boundary, rather than only fabricated snapshots.
- PostgreSQL-only opt-in fails closed before the legacy TestMain initializer. The final gate rejects omitted required cases and skips; examples remain disabled and deployment fixtures protect runtime secrets.

## Issues

### Critical (Must Fix)

None found in the inspected paths.

### Important (Should Fix)

#### I1 — A definite proof refusal permanently locks this tab into an unknown operation

- **Location:** `web/src/features/system-settings/catalog-sync/index.tsx:231` (error classification), with binding persistence at line 331 and lock derivation at line 265.
- **Trigger/effect:** after verification returns, the UI persists a new operation binding before sending apply. If the server rejects that apply with `SECURITY_PROOF_EXPIRED`, `SECURITY_PROOF_METHOD_MISMATCH`, or another definite pre-mutation proof error, `onError` retains the binding because it clears only `CATALOG_CONFLICT` and `CATALOG_PLAN_BLOCKED`. There is no receipt: `middleware/secure_verification.go:55` returns the proof refusal and `controller/catalog_sync.go:341` returns before `ApplyCatalogSyncPlan`. Receipt lookup consequently returns not found; `unresolved` keeps preview, confirmation and restore disabled. Reloading the page restores the same binding from sessionStorage, so it does not recover the workflow. A suspended/delayed tab or a changed verification policy can reach this condition without any catalog mutation.
- **Why it matters:** a normal security refusal becomes a persistent UI dead end labeled “Result unknown,” although the server established that this request never reached the write. The operator cannot obtain a fresh preview/proof through the same view. This violates the planned error-recovery workflow; backend proof rejection itself is correct.
- **Minimal fix:** classify explicit, authoritative pre-write refusal codes separately from uncertain commit/network/publication outcomes. Clear the unused binding and its stored value for those refusals, retain or invalidate the plan as appropriate, and let the operator obtain fresh verification. Do not clear an unknown write merely because an immediate lookup returns 404; preserve uncertainty handling for lost responses and `CATALOG_COMMIT_UNKNOWN`. Add a frontend regression that returns an actual shared proof-rejection code, verifies the binding is cleared across remount and confirms a fresh user-driven verification can proceed. Keep the existing unknown/pending no-resubmit regressions.

### Minor (Nice to Have)

#### M1 — Map leaves bypass raw null rejection

- **Location:** `internal/catalogtransport/transport.go:189`.
- **Issue/effect:** `exactWireFields` recursively checks structs/slices, but accepts an entire map as a non-null leaf. A coverage member such as `"vendor": null` decodes as integer zero and can match an actually empty vendor kind. This is a genuine strict-wire gap, not evidence that a nonempty kind can evade coverage or that a partial source can delete managed entries: typed coverage equality, required capabilities, content digest, authenticated source, and later nonempty-source checks remain enforced.
- **Minimal fix:** recurse through map values using the authoritative element type and reject raw null leaves. Add a focused transport fixture with a valid empty-kind snapshot whose zero coverage count alone is replaced by null; require rejection. Keep valid numeric zero accepted. This is a small robustness correction, not a reason to weaken or redesign manifest validation.

#### M2 — Restore-oriented changelog copy is still hard-coded to dev

- **Location:** `web/src/features/system-settings/catalog-sync/changelog.tsx:313` and line 168.
- **Issue/effect:** the reusable changelog says “Use dev for entire item” and “After sync” without knowing plan kind. The current restore engine rejects local drift before returning a plan (`model/catalog_sync_restore.go:69`), so the misleading restore *conflict checkbox* is not demonstrated reachable through the current production restore API. The restore value heading is reachable. This is wording/defensive reuse debt, not a proven authorization or inverse-data defect.
- **Minimal fix:** supply plan kind and use inverse/restore wording where applicable; preserve ordinary sync wording. A restore-render test can guard the text. Do not introduce restore conflict-overwrite behavior merely to exercise a checkbox the backend intentionally does not support.

#### M3 — Consequential keyboard/focus coverage remains missing

- **Location:** `web/src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx:950`.
- **Issue/effect:** the named keyboard test tabs to and activates preview, then checks locales. It does not establish keyboard access to deletion consent, save/final-confirm transitions, focus transfer to verification, or focus return after cancellation. Reuse of shared dialogs is helpful, but does not prove this feature's composed flow. This is a bounded verification gap; I did not observe an actual inaccessible control.
- **Minimal fix:** extend focused tests through keyboard-only confirmation and cancellation with focus assertions, including deletion consent and the verification handoff. No general dialog refactor is required.

#### M4 — Expected diagnostics obscure clean regression output

- **Location/evidence:** `controller/catalog_sync.go:491`–503 intentionally records unknown actors without inventing a role; the preserved controller HTTP gate emits the disclosed actor-role diagnostic. `/tmp/catalog-task8-web-test.log` contains navigation/scrollTo diagnostics, and the Task8 report records Gin startup and prior malformed-repair/setup diagnostics.
- **Issue/effect:** these are expected diagnostics rather than failed assertions, but make new unexpected failures harder to distinguish. They do not justify a fake administrator identity or global production-log suppression.
- **Minimal fix:** optional test-local capture/assertion of expected diagnostics and local stubs for unsupported jsdom operations. Preserve raw historical logs and production audit semantics. Failure audit remains best effort; do not claim guaranteed durable failure records.

## Explicit disposition of deferred and parked categories

1. **Driver/audit/Gin/jsdom/setup noise:** retained and nonblocking as M4. The deliberately killed MySQL-connection diagnostic is historical, outside current PostgreSQL verification, and was neither reproduced nor counted as a current PostgreSQL failure. Ordinary malformed repair and setup not-found messages are consistent with the exercised negative paths. Unknown actor role stays unknown; failure audit is best effort.
2. **Deferred group-index lock:** addressed. `model/pricing.go:628` checks context and uses `TryLock` for the lifecycle projection, returning `ErrCatalogWriterBusy`; `publishCatalogRuntimeAndOptionsGuarded` propagates rebuild errors before durable success acknowledgement. Ordinary projection retains its normal lock. The cross-task publication chain does not discard this error.
3. **Map-null wire strictness:** confirmed, M1. No demonstrated completeness/deletion bypass. Inner `Entry.Value` canonicalization remains the original manifest semantics; this review does not certify arbitrary nested raw JSON as a second strict wire schema.
4. **Restore copy/accessibility/module size:** M2/M3. The 577-line lifecycle/view module is an optional maintainability improvement, not a required extra refactor. The original StrictMode defect is fixed by a pure representative-by-unit map; repeated resolution uses `currentPlan.digest` while retaining original changes for controls, so the original stale-second-digest defect is not present. I1 is a different lifecycle integration issue.
5. **Retained billing arithmetic:** classified as pre-existing math defects, not introduced regressions and not fixed by coherent capture. The supplied adaptor diff retains `outputPrice*n + inputPrice*inputCount`; generic image helper still adds the quantity ratio. The characterization at `controller/catalog_image_pricing_test.go:402`/415 retains 350,000 reservation and 210,000 settlement for the described old-rate case; new-rate figures are 220,000/120,000. Zero direct image rates remain zero at final settlement in the sampled test but can reserve the positive anchor first. The Seedance adaptor retains `price / (2 * modelRatio)`, and `relay/catalog_task_pricing_test.go:261` explicitly verifies the existing positive-only multiplier limitation (217,800 reserve, 2,000 terminal charge for the described zero-rate case). These must not be described as correct free-price/quantity mathematics. Fixing them changes separately retained billing behavior and is outside this sync implementation's approved capture-only scope; they deserve explicit follow-up before relying on those pricing modes operationally. The full submit evidence uses real persistence and terminal consumers but controlled local upstream and supplied identity, not a paid provider or full HTTP middleware claim.
6. **Representative upgrade and bounded load:** acceptable for the approved, explicitly bounded verification requirement. The fixture independently describes representative rc40 Model/Vendor rows, then runs current `InitResources` twice and preserves IDs/data/indexes. It is not a complete released database or old-binary execution. The restricted-role 30k-history/15k-active scan and forced timeout prove bounded local rollback/lock release, not production capacity. O(all-history) work and broad relation fences remain operational scaling constraints; production sizing is unknown, so deployment readiness at arbitrary scale is not established.
7. **Historical wrong-engine initialization:** corrected in final evidence, not erased. The explicit nonempty opt-in branch and invalid-value failure precede the legacy model TestMain initializer; the final selected gates use it. Earlier SQLite-initializing commands cannot support PostgreSQL-only claims. No new SQLite/MySQL matrix, minimum-version acceptance, unrelated full-Go-suite acceptance, Redis-server verification or production deployment claim is made here.

Other ledger observations encountered: the earlier cancellation/pin-release issue is addressed by keeping the pin until reserved-connection cleanup; the ordinary canonical-no-op candidate publication is present; the target namespace precommit proof is present and distinct from source postcommit read proof; the explicit ordinary model-removal alias preparation is present. These are reviewed integrations, not reopened historical work. No other still-open parked blocker was identified in the deferred/parked ledger lines inspected.

## Spec compliance

**Verdict: With fixes; not fully complete as submitted.** The inspected implementation substantially satisfies the approved managed-source, ownership, deletion, proof binding, transaction, singleton eligibility and publication/recovery contracts. PostgreSQL-only evidence is substantial and honestly bounded. I1 leaves the required user-facing error-recovery flow incomplete, while M1–M3 are disclosed wire/UI/testing gaps.

No production dependency additions appear in the supplied change inventory. Web sync writes are restricted to catalog/price/baseline/operation state; ordinary channel-removal behavior remains separately owned by its existing action. Runtime examples do not grant production apply authority. Deployment still depends on correct unique manual instance identity: the heartbeat gate explicitly cannot discover two processes impersonating the same manual node, and this is not distributed publication support.

## Recommendations

- Route I1 through the single consolidated fix wave with a focused frontend regression and scoped re-review; do not replace unknown-commit handling with blind retry.
- M1 and M2 are small, bounded improvements suitable for that same wave if approved; M3 can be covered without broadening implementation. M4 and the module split are optional maintenance.
- Preserve the current conservative deletion/reference policy and clear operational limitations. Do not turn retained arithmetic defects, fixture limitations or noisy logs into claims of broader correctness.

## Declined to judge

- SQLite/MySQL behavior, including historical driver-kill diagnostics: explicitly outside the latest PostgreSQL-only verification scope.
- Correctness repairs for the retained Grok quantity, direct-image reservation and Seedance zero-multiplier arithmetic: separate existing billing behavior; this review judges capture/preservation and reports the defects rather than authorizing a formula change.
- Full released-schema/old-binary upgrade coverage: the approved acceptance was representative catalog upgrade; the actual fixture cannot establish broader release compatibility.
- Production load capacity or arbitrary history-scale latency: only one local 30k/15k measurement was supplied and no production sizing is available.
- Cloud CI execution, unrelated full Go suite and live Redis-server behavior: not executed by the supplied final local gate or this review.
- Real dev DNS/TLS/credentials, paid upstream behavior, live target configuration, deployment, first production synchronization and recovery in production: no live execution or corresponding authorization in this review.
- Multi-instance publication consistency and duplicate processes impersonating one manual node: v1 is explicitly single-instance; the implementation does not claim a distributed barrier or deployment discovery service.
- A lifecycle/view module split: optional maintainability work, not a missing functional requirement or permission to expand this patch.
- Exhaustive correctness of every uninspected test/locale/setting-table line and unrelated consumer: outside the actual bounded coverage described above; prior reports are evidence, not a substitute for claiming a read that did not happen.

## Assessment

**Ready to merge? With fixes.**

Counts: **0 Critical, 1 Important, 4 Minor.** The core safety architecture is strong in the inspected paths, but a definite security refusal currently leaves the shipped UI persistently unusable in the same tab. Fix I1 and verify the narrow lifecycle behavior before merging; retain the explicit billing and operational limitations in delivery.
