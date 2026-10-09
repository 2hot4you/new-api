# Release review

User confirmed fixing the stale frontend assertion, verifying the release, backing up production PostgreSQL, then publishing develop to main and following all four deployments. No external Antigravity/Claude executor used, per the user's explicit restriction.

The production pricing component intentionally changed its label in 958a4eecc. Updated only the model-listing test name and expected label to `Custom pricing expression`. Preserved assertions that an unrecognized expression does not invent a price and that opening its details retains the full expression source. No application or billing behavior changed in this fix.

Fresh verification on 2026-10-09:

- `bun run test`: 290 files / 2731 tests passed, exit 0.
- `bun run typecheck`: passed.
- `bunx oxlint -c .oxlintrc.json src/features/models/__tests__/model-listing.test.tsx`: passed.
- `bunx oxfmt --check src/features/models/__tests__/model-listing.test.tsx`: passed.
- `VITE_SITE_PROFILE=molii bun run build`: passed.
- `make test`: root Go module and relaykit passed.
- `git diff --check` and `sh -n` for the task's backup helper: passed.

Four production backups created using private remote runtime configuration and the running application's network. Backup files are permission 0600 in private directories. Each nonempty custom-format dump passed `pg_restore --list`; no restore drill performed. Previous image reference saved next to each dump. iXiaozu's first attempt lacked the database CA mount; reran with the existing certificates mounted read-only and verified successfully. The incomplete initial directory is retained and is not a valid backup.

| Site | Verified backup directory | main.dump SHA-256 |
| --- | --- | --- |
| Molii | `/opt/molii/production/backups/new-api-before-main-20261009-314266139.Sv5pKF` | `930616d74672927546c97440e343b6ea04c6ec518f2f34cd2be4234a4df8455d` |
| iXiaozu | `/opt/ixiaozu/production/backups/new-api-before-main-20261009-314266139.7oLLYp` | `e07673938e0ac29afbd4f9ae9873f9078e6e1b1f908ca42a537fbb99b1c57c80` |
| Claudeye | `/opt/claudeye/production/backups/new-api-before-main-20261009-314266139.iROTAu` | `8d09922e88beb331ebd253f2bd3881839399167b68e8a771c38f0bd62e773131` |
| Model Claudeye | `/opt/claudeye/production/backups/new-api-before-main-20261009-314266139.dmRZSi` | `393027476cc0078162c23156a9537c6f7db30865ae6a98212e243b2ecaa8a787` |

Before publishing, fetched main/develop: main a103b6d8 is an ancestor of develop 314266139; 198 develop-only commits and no main-only commits. Existing primary checkout and unrelated untracked ` 2.ts(x)` files are preserved. Publication uses a normal fast-forward push from the verified linked worktree; no force push.

Release commit `3a0f2d30f8f6dad8526c91cdf0c2eafb3b1c8d7d` (`test: align model listing custom pricing label`) atomically fast-forwarded both remote develop and main. Verified both remote refs resolve to that exact SHA.

## Deployment blocker

No push-triggered Actions runs or commit check runs were created for the release. Attempted the existing, authorized workflows with `gh workflow run deploy.yml -R 2hot4you/new-api --ref main -f target=all-production` and the equivalent `docs-deploy.yml` command. Both returned HTTP 422: `Actions has been disabled for this repository.` No successful dispatch or new run exists.

Read-only API checks confirm the repository is not archived/disabled, current credentials have admin/push permissions, repository Actions permission is enabled, and both workflow states are active. These settings do not explain the platform's dispatch rejection; no account-level restriction or billing cause can be confirmed. No repository security settings or billing settings changed, and no direct SSH application deployment bypassed the CI checks.

Public `/api/status` checks on all four sites returned success=true and their previous production versions ending in `a103b6d8ffc4`. Thus branch promotion is complete, production publication is not complete. Task remains blocked and unarchived until Actions runs are restored and the intended release is verified. Existing local primary main branch and untracked user files remain untouched.
