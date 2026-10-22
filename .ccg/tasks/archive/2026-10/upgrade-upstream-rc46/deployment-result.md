# Development rollout

## Final development outcome

Accepted development upgrade completed. Run39762321755 is fully successful, including verify, actual offline PostgreSQL main/log backup restore, immutable deployment, repeated startup and release summary. Target artifact /tmp/molii-rc46-release.Qnlsmn/deploy-result-development.json states development/dev.molii.co, success, source_sha=cf9b53e2a101e70ff743ec238fca662b0fc856ed, rollback=not_attempted.

Actual public status: development-cf9b53e2a101, success=true, quota_display_type=CNY, usd_exchange_rate=7. Compared normalized public pricing before/after: all46model prices and all10vendors exactly equal, no changed/removed entries. Includes raw expressions/currencies and legacy price components; this is a public-projection comparison, not a claim that private DB bytes or paid-provider settlement were independently compared. /pricing, /pricing/gpt-image-2 and /pricing/deepseek-flash each returned200; no assertion of authenticated UI or real paid requests.

After development acceptance, origin/develop fast-forwarded c244→cf9b53e2a with no force. That ordinary push triggered additional same-application-source run https://github.com/2hot4you/new-api/actions/runs/39767173903, currently in progress. Its newer-workflow Bun1.4.0/tokenkit verification is NOT yet claimed green. The actual source is already deployed and accepted by run39762321755; record the additional run as pending and require it green before any separately authorized production promotion. Task scope closes at the completed development rollout and develop synchronization; no production work is authorized.

Origin/main remains936aade39dcbb79071473a20cc16df96fcdd3e8a. No live prices, metadata, vendors, credentials, catalog apply or production deployments were changed. Reused worktree and user's untracked duplicate/old-task files are preserved. Closeout commit contains task records only and is excluded by the existing deployment paths-ignore, not an application change.

## Chronological evidence

Candidate: cf9b53e2a101e70ff743ec238fca662b0fc856ed, pushed to origin/upgrade/rc46. Real two-parent merge of reviewed Molii business base and official rc.46.

Run: https://github.com/2hot4you/new-api/actions/runs/39762321755

Dispatched2026-10-22 10:20:10 UTC via trusted develop workflow c2442919e29403123cf8e5bcd15eea4db4dcc3e6. Its displayed headSha is the workflow source, not the candidate source; source_ref pins cf9b53e2a101e70ff743ec238fca662b0fc856ed. Target development only, backup_postgres=true, verify_repeated_startup=true.

Status at creation: in progress, not deployed/accepted. The later final outcome above supersedes this historical status.

Observed progress: trusted-source prepare, immutable candidate checkout, frontend types/full tests/i18n/build, root/relaykit vet/build and complete normal PostgreSQL acceptance passed. Race acceptance is still running. The trusted pre-upgrade workflow installs Bun1.3.14 and does not contain explicit tokenkit vet/build steps; the candidate's acceptance runner nevertheless executes independent tokenkit normal/race. Candidate Docker builder uses pinned Bun1.4.0 (verified locally). A subsequent approved develop fast-forward will use the upgraded workflow including Bun1.4.0 and explicit tokenkit vet/build; do not mistake this initial trusted-workflow run for that newer workflow's acceptance.

Pre-deployment public API check: development-c2442919e294 remains healthy. Captured46public model price projections and10vendors for non-mutating after-deployment comparison, including explicit DeepSeek CNY holiday-aware expressions, Claude USD and GPT Image2 USD. No pricing was saved. Task-created local PostgreSQL/Redis test containers and their disposable volumes were removed after all local tests finished; user development containers were not touched.

Verify job succeeded29m26s; observed actual immutable-source log counts are326frontend files/3053tests, full normal/race root43/2685+relaykit9/568+tokenkit1/7, final mandatory PostgreSQL1644tests/subtests/40exact selections in EACH mode,28deploy Python tests/192shell assertions. No required test skipped. Release job119781547622 is currently building the development image; backup/deployment has not yet run.
