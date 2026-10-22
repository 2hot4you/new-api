# Release checkpoint

Updated 2026-10-23 Asia/Shanghai after push. Latest human authority: repair, commit/push develop (called dev), then merge/push main only after development CI/CD is all green. This is the new repair task, not the completed 915723524 release.

## Exact source

- Worktree: `/Users/naf/Documents/Codex/Projects/molii.co/new-api/.worktrees/diagnose-temporary-asset-expired`
- Branch: `fix/pricing-editor-catalog-errors`
- Base: `915723524c9a96bb70860c4905580b3a28a13e9e`
- Pushed develop: `9a27a34a4bf3d1cae79b639a48f00270866ad96d`
- Push confirmed `915723524..9a27a34a4 HEAD -> develop`.
- Develop push CI: https://github.com/2hot4you/new-api/actions/runs/39815972544 , attempt 1, queued at first observation. Resolve deployment targets / Verify backend and frontend jobs present. No green acceptance yet.
- Main remained base 915723524 on immediately pre-push fetch. NOT promoted.
- Required development version: `development-9a27a34a4bf3`, success=true. First public check still reported healthy `development-915723524c9a`, consistent with queued new release; not accepted as new-source deployment.

## Verification / boundaries

Read verification.md and both final reviews for completed local gates. Normal and race PostgreSQL gates both exit 0; frontend 328 files / 3082 tests pass; deployment Python29 + shell192 pass; build/types/i18n and Go vet/build pass. Application tree unchanged after reviews; final commit contains reports/spec feedback only.

Owned temporary test PostgreSQL and Redis containers were validated by exact IDs then stopped; --rm removed their disposable fixture data. They held no user/business data. Existing user development and all production containers were untouched. Logs and recoverable old node_modules symlink remain in `/tmp/molii-pricing-repair-verify.Wdi5yR/`.

## Monitor / next action

### 2026-10-22T17:29Z completion / main promotion

- Develop push run39815972544 attempt1 completed success. Resolve119954533988, full Verify119954618936, actual Release development119969046291 and Summary119972545459 all success. Release completed at17:16:26Z; immutable-image deployment, target result recording and upload succeeded. Result artifact12445056139 is named `deploy-result-development-9a27a34a4bf3d1cae79b639a48f00270866ad96d`.
- Independent public status check returned success=true and exact `development-9a27a34a4bf3`.
- Fresh fetch confirmed origin/develop=HEAD=accepted9a27a34a4bf3d1cae79b639a48f00270866ad96d, origin/main=reviewed915723524c9a96bb70860c4905580b3a28a13e9e and ancestry valid. No intervening commits. Main fast-forward push succeeded, preserving the exact accepted source SHA, without modifying another checkout or including local checkpoint changes.
- Main automatic push run39823030685 triggered and is in progress for exact same SHA: https://github.com/2hot4you/new-api/actions/runs/39823030685 . Four production targets follow the unchanged workflow; their completion is NOT claimed or required for this task's authorized terminal criterion.
- Authorized release criterion reached: dev CI/CD all green + exact live version, followed by main merge/push and new run confirmed. Archive this task in a local-only audit commit; do not push that new audit SHA. Stop automation-2. No business price/config/metadata/apply/restore write was performed.

### 2026-10-22T17:12Z observation

- Exact-source run39815972544 attempt1 remains in progress. Resolve job119954533988 and full Verify job119954618936 both succeeded, including normal/race gates and deployment contracts.
- Release development job119969046291 is building/pushing the target image. SSH/deployment/result steps have not run; no failed step observed. This expected running state does not justify a retry or dispatch.
- Public status still success=true, version `development-915723524c9a`; required9a27 version not deployed yet. Do not promote main until Release succeeds and the exact live version matches. No live write or main push this check.
- Next: continue checking this same run/attempt and development version on the next scheduled heartbeat. No actionable failure/user notification.

### 2026-10-22T16:58:29Z observation

- Run39815972544 attempt1 remains in progress for exact develop source9a27a34a4bf3d1cae79b639a48f00270866ad96d.
- Resolve job119954533988 succeeded. Verify job119954618936 is running Race Detector; frontend/types/i18n/build, all Go vet/build and normal PostgreSQL step succeeded. Deployment-contract step pending; Release development has not started. No failed step observed.
- Public development remains healthy at old `development-915723524c9a`; new deployment is not yet accepted. No main promotion, dispatch, retry or live write performed.
- Next heartbeat: inspect the same exact run/attempt and real Release development plus required live version. Existing in-progress state is expected; no actionable failure/user notification.

Existing matching heartbeat automation-2 was updated to the new task/source/run and ACTIVE, every15 minutes. Old 915 source / prior four-production completion instructions were replaced. Preserve original thread and interval. No duplicate automation or workflow dispatch.

1. Inspect explicit repo exact-source run jobs/attempts. Both Verify and Release development must succeed; summary/old runs do not prove acceptance. Read failure logs and minimally fix/test/review if necessary, then update actual source/run and monitor instruction.
2. Read public development status and require exact source version. No paid request, live price save, apply/restore, SQL repair, quote deletion/conversion or guessed metadata.
3. Fresh fetch: require origin/develop exactly accepted source and origin/main an already-reviewed ancestor. Do not include new unverified changes or force push. Fast-forward exact accepted source to main only then.
4. Record main actual SHA and new automatic release link. Latest task ends after main push / auto-deployment trigger, not after all production releases. Archive CCG task with a local audit commit and stop this monitor; do not push a new audit-only SHA that differs from accepted deployment.

Checkpoint updates during monitoring may remain local until final archive to keep accepted source SHA stable. Preserve all other user/worktree edits and original live pricing-page draft. Unknown errors remain generic; the incompatible fixed-Grok/xAI task-usage quote remains rejected until separately authorized pricing ownership remediation.
