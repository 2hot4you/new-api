# Implementation and runtime checkpoint (task not complete)

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
