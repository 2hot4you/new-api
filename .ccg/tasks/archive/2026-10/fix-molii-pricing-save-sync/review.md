# Implementation, runtime checkpoint, and completion receipt

## Frontend

Two disjoint workers implemented the approved guards. Upstream individual and bulk controls omit four dedicated Seedance models with existing translated guidance; eligible counts, stale selection resolution and confirmation/save exclude them. Generic buildPricingChanges and reset filter them; mixed JSON edits save ordinary values with explicit unsaved dedicated-model guidance, protected-only operations do not mutate or report success.

RED evidence in reports/task-1.md and reports/task-2.md; final integrated 8-file related suite: 72 passed (19:20:22 local, 9.90s). Typecheck, changed-file lint, i18n completeness and production build passed. Two independent read-only reviewers reported no code blockers. External antigravity/Claude wrapper unavailable; no external dual-model verification claimed.

Full local web suite attempted with maxWorkers=4; unchanged lobe-icon.test.tsx again had five timeout failures (named variant/accessibility, changed name, base fallback, Gemma.Simple, LobeHub.Morden). Remaining owned worker stopped making progress for over nine minutes; only its verified exact process IDs 40593/38537/38531 were terminated (exit143). Do not claim full-suite completion or success. Those icon modules were not changed.

## Runtime preflight / rollback

Source and target DB identities distinct; zero active tasks, one manual master instance each. Existing manual NODE_NAME preserved independently in each DB. Cached PostgreSQL15 helper, strict read-only PGOPTIONS and READ ONLY transactions used; no migrations or application database writes. Both deployment locks and existing immutable Compose image/config equivalence verified before recreation.

Operational review found and fixed explicit Compose error propagation, actual restored fingerprint/no-catalog-env validation, preexisting config detection, EXIT/signals rollback, pre-mutation topology drift check, source-before-target verification and a bounded authenticated-browser target-ready ACK gate. Both reviewers then found no remaining blockers. --check dry run passed without configuration changes.

Approved provisioning attempted: source env installed and only development container recreated with the same image. Unique reader verifier source/secret target generated privately, never printed or saved to repo. Fixed internal/catalogtransport GET export from the target network returned unavailable. Source log confirms authenticated export HTTP503 at 2026-10-22 19:28:23 local. Target recreation was NOT reached. Automatic rollback restored development runtime env and original image/noncatalog configuration; rollback failure=false, healthy. Production never recreated/modified.

Read-only source completeness query subsequently found two orphan price entries:
- billing_setting.billing_expr -> claude-opus-4-6
- billing_setting.billing_mode -> claude-opus-4-6

No exact active metadata with reliable USD/CNY currency exists for that model. ExportManagedCatalog rejects any model price without exact currency metadata (model/catalog_sync_snapshot.go). Removing prices or inventing metadata is outside the approved no-data-change configuration scope. Do not weaken the completeness check or silently drop entries. User must choose metadata restoration vs backed-up legacy-entry cleanup before another enable attempt.

## Artifact / release boundaries

No push/merge/code deployment, no pricing sync preview/apply/restore POST, no paid generation or other sites changed. Private inactive staged credentials/probe removed after rollback; original 0600 runtime backups retained on each Molii deployment directory. Runtime sync remains disabled pending data decision.

The SDD helper's basename workspace collided with an existing tracked task-2 report; this task's worker reports were copied into this task's own reports/ and the prior tracked artifact restored byte-for-byte with apply_patch. Unrelated artifacts are unchanged. This task stays active, not archived as completed while provisioning is blocked.

## Approved remediation and completed provisioning (subsequent user turn)

User explicitly confirmed the model is unused and authorized backing up/removing those two DEV leaves, then committing/pushing. Both native independent reviewers rechecked the offline SQL and wrapper. External antigravity/Claude wrapper remains unavailable; no dual-external-model review claimed. Review findings fixed before server execution: pg_temp-qualified temporary tables (including COPY), actual stopped-state verification before restore, distinct DB identities, durable CSV+checksum filesystem sync, checksum verification before restore, raw-original read-only reconciliation before any CAS restoration, and creating/loading temporary data before opening the read-only transaction. No catalog revision/digest/publication fields are manually changed.

Real isolated local PostgreSQL15 fixture, no host port or application DSN: initial no-op RED failed the exact-deletion assertion; same-name public-table RED failed scope validation; final GREEN verifies exact two-leaf deletion, unrelated option raw values and coordination anchors unchanged, byte-exact restoration, no-op restoration, raw-original checker true/false/true, same-name permanent tables untouched. Changed backup/current values, missing row, duplicate JSON key, pending catalog state, enabled model reference and additional orphan all fail without option changes. Shell syntax and git diff checks pass.

DEV cleanup succeeded on the server, with original container and image restarted healthy. Raw two-option backup retained privately:
- /opt/molii/development/data/catalog-sync/legacy-price-cleanup-iL9JpP/options.csv
- mode0600, owner molii-deploy, 9172 bytes
- SHA256 38264e8f0a66f66b2dda37414b98336f01d95646283293b1869a3221d4250c0e

The transaction verified exactly two updated rows, exact expected JSON postimages and no other option raw-value changes. Follow-up completeness query reported zero orphan price entries. Production DB was not used for this cleanup. Removing this unused model's custom expression/mode does not create a general routing prohibition; do not later route that retired name expecting the deleted override.

Both deployment locks and fresh immutable Compose/live equivalence preflights passed. Runtime provisioning then succeeded source-first, target-second, preserving each current image and noncatalog environment/mounts/ports/networks. The actual internal/catalogtransport.Fetch probe succeeded twice (source_id dev, validated true). Both public /api/status checks passed. Logged-in Molii environment-sync panel visibly showed role target, source dev, target molii, target ready yes, management ready yes; its bounded ACK was then issued. The page also displayed a Dev update preview during concurrent human browser use; agent did not click review/confirm/apply or submit any catalog price mutation. Root GET /api/catalog_sync/status returned200.

Private runtime backups retained:
- /opt/molii/development/.env.runtime.before-catalog-sync-20261022T120358Z
- /opt/molii/production/.env.runtime.before-catalog-sync-20261022T120358Z

No price sync apply or restore executed, no paid generation, other sites or Docker daemon changes. Generated credential stage was removed by the successful provisioner; actual reader token/verifier remain only in private runtime files, never repository/output.

Final read-only server check: both original immutable image IDs still healthy; runtime files0600; development role source/dev, production role target/dev/molii, single-instance mode true. Production catalog_sync_operations count0 and baseline_generation0 confirm no catalog apply. One-off uploaded tools/probe removed; all raw/runtime backups retained and CSV checksum revalidated.

Fresh frontend revalidation: 8 files /72 tests passed at19:58:44 (13.54s); typecheck, exact-eight-file lint, i18n completeness and production build passed. A broad lint command inadvertently retained the package script's dot and reported preexisting errors in unchanged files (utils, http-status-code-rules, confirm-dialog, assets, scripts, etc.); the corrected scoped lint is clean. The earlier full test run's five lobe-icon timeouts/hung worker remain a known broad-suite limitation, not a green full-suite claim. No application source changed after its prior dual-independent review. User authorized pushing current fix/molii-pricing-save-sync only; no additional branch merge or code deployment.
