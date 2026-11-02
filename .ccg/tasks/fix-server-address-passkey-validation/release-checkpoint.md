# Release checkpoint2026-11-02

## Important: first candidate rejected2026-11-02T20:00+08:00

Do NOT promote d40cf5c5c, regardless of any early green checks. Local full audited normal acceptance failed at unchanged TestCatalogMixedPasskeySave:79: expected second ServerAddress change rejected under stored explicit origin, got nil. Whole-branch retainPrevious gate prevented storing the NEW first-setup origin. Corrected working diff retains the original materialization branch, conditionally omits only unusable previous origin, and always stores the new origin. New controller test independently asserts preview/storage origin and foreign second-save rejection, observed RED on d40 before fix. SQLiteall25 + four existing mixed Passkey PG contracts GREEN; PG/MySQL all25 and mixed PG race in progress. Two independent analysts confirmed cause; fresh follow-up reviews required before corrected push. Production/main untouched. Exact corrected source/run to replace failed candidate below after push.

- Source pushed to develop: **d40cf5c5c75722136fec81cd80d8fda1d7876ed2**.
- Push workflow: **41439073648**, https://github.com/2hot4you/new-api/actions/runs/41439073648 . First observed in_progress; no main merge yet.
- Development acceptance expected version: **development-d40cf5c5c757**. Require actual Verify backend and frontend plus Release development success and public status exact match.
- Main baseline: ce7c4eb362728f3d55c72e80255f36829483b0a9. Normal merge required (main-only historical merge ancestor), not force-push or overwriting main. Previously accepted develop base1f7724f74 includes rc47 and app/docs aigc exclusions.
- Automatic main production targets: production-molii, production-claudeye, production-model-claudeye. **Never production-ixiaozu** in this batch. No separate dated maintenance approval received.
- Heartbeat successfully created: **automation id `aigc`**, name「完成服务器地址修复发布（排除 aigc）」, ACTIVE,15-minute interval. ID is misleadingly short; it is this task's monitor, not authority to deploy aigc. Do not duplicate it. Pause/delete only this monitor when three production targets and triggered docs have actually passed.

Local audit acceptance still running: session97422, .superpowers/sdd/plan-fix-server-address-passkey-validation/postgres-ci-normal.log. Complete independent root/relaykit/tokenkit suites passed; no mandatory group failure observed, currently helper-ci-legacy-regressions. Do not archive or claim broad green before final exit. Focused real PG race PASS95.314s; code vet PASS.

After local verification is finished, verify ownership labels and remove only task-created disposable containers server-address-test-postgres-20261102, server-address-test-mysql-20261102, server-address-test-redis-20261102 (label codex.task=fix-server-address-passkey-validation). Never touch molii-new-api-dev-postgres/redis. Preserve ignored evidence until copied into final verification, and preserve all unrelated dirty task deletions/untracked files. Do not push task-only audit updates as another deployment source.

No production setting written: after the new production version is live, user must save ServerAddress=https://model.claudeye.com again. Existing frontend invalidates status and API-address display automatically after a successful save. Do not claim the production value has already changed.
