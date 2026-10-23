# Read-only diagnostic findings

## Observed live evidence

2026-10-23 Asia/Shanghai, user-owned Chrome pricing tab241829902 showed `production-molii-9a27a34a4bf3`, qwen filter, no matching local rows and a pending override-removal Save action. Did not refresh, click Save/Delete/Reset, navigate that tab, or discard drafts. Created separate metadata lookup tab241829905, navigated to existing `/models/metadata`, searched `qwen` using the visible search input and Enter. Fresh result URL `/models/metadata?filter=qwen` showed `未找到模型` and total0. Closed only the agent-created lookup tab. This establishes absent currently listed Qwen metadata, not the commercial currency/provenance of any leftover quote.

## Independent frontend analysis

Native explorer `/root/analyze_qwen_delete_frontend`, read-only, no files changed. Exact Chinese message originates from `web/src/features/model-pricing/api.ts:88`, invoked by `buildPricingChanges` before the PATCH at `ratio-settings-card.tsx:512-525`. Remove action locally deletes per-model maps at `model-ratio-visual-editor.tsx:463-505`, but the save builder applies the same exact-metadata/known-currency predicate to all dirty changes, including deletion (`api.ts:232-245`). Existing `pricing.test.ts:334-355` expressly tests missing-metadata deletion rejection. Removal entry checks builtin-only configuration, not metadata eligibility, so the UI permits a draft it later refuses to save.

Baseline can also be stale, but fresh live metadata search here is empty; this is not established as a stale-tab-only issue. Draft retention on rejection does not promise persistence across reload. Original tab remains untouched.

## Independent backend analysis

Native explorer `/root/analyze_qwen_delete_backend`, read-only, no files changed. `UpdateModelPricing` rejects missing exact metadata before processing Reset (`model/model_pricing_config.go:605-624`). Reset restores compiled default map values, not universally pure deletion. Every accepted ordinary change queues currency updates; the final exact metadata postimage check at821-834 also rejects missing rows. Missing metadata read projection uses compatibility USD (`model_billing_money.go:47-50`), while captured write currencies contain exact rows only (`catalog_sync_runtime.go:288-296`), so blanket bypass also causes version mismatch and risks inventing currency authority.

Prospective runtime staging permits historical orphan pricing and removal (`catalog_sync_candidate.go:44-57`); managed export remains stricter. A deletion-only cleanup can therefore be supported narrowly through existing staged transaction/publication machinery, but ordinary writes must remain strict.

## Conclusion / proposed bounded follow-up

Confirmed unsupported cleanup scenario in the previous repair: frontend and backend intentionally share write prerequisites, leaving no usable removal path when exact metadata has already disappeared but persisted pricing remains. Do not tell the user to create guessed metadata merely to delete pricing. Do not simply remove frontend guard or infer USD from the read fallback.

If approved, support explicit deletion-only removal of the named model's stored/provider pricing overrides, no metadata/currency writes or default-price insertion, with authoritative stale-version and whole-catalog rechecks, existing transaction/revision/publication protection and batch atomicity. Preserve normal add/edit/currency prerequisites, dedicated Seedance exclusions, unrelated prices, and compiled default fallback semantics. Removing overrides is not deleting a model or hiding builtin defaults.

Regression scope would include missing-metadata removal success, orphan add/edit/currency rejection, stale version rejection, mixed-batch rollback, provider leaves, unrelated preservation, no metadata creation, defaults, runtime publication and real three-database acceptance if backend persistence behavior changes. This diagnostic made no such implementation change and does not claim that verification.

## Verification and boundaries

- Coordinator executed exact existing frontend `bun run test -- src/features/model-pricing/__tests__/pricing.test.ts`: exit0, one file15 tests passed, including current deliberate missing-metadata deletion rejection. This reproduces the current contract, not a fixed behavior.
- Backend analyst executed existing targeted tests: currency snapshot/prerequisite and rollback, pricing-options orphan removal and historical-orphan recovery passed on SQLite3.50.4. MySQL/Postgres children skipped (no DSNs); no cross-engine/full backend green claim. An additional existing model-pricing-save test failed on its explicitly configured healthy-single-instance fixture precondition, not the reported branch; not changed or hidden.
- External antigravity/Claude wrapper absent; the two native independent analyses above are substitutes and not external-model review claims.
- Preserved all turn-start unrelated tracked deletions and untracked duplicate files. Only this task's records are staged/archived locally; no app edits, production changes, secret extraction, paid requests, commits to develop/main or pushes.
