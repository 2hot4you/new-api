# Backend/data conflict audit: Molii develop → upstream rc.36

## Scope and evidence

Read-only source/history audit; no merge, source edit, database access, or test execution. Findings describe upgrade risks, not demonstrated production failures. `.ccg/spec/` does not exist in this worktree. All source citations below identify either **Molii** (`origin/develop`) or **U36** (`v1.0.0-rc.36`); line numbers refer to that revision, not a future merged file.

Frozen refs:

| Ref | Commit |
| --- | --- |
| origin/develop | `e4120574b69e7dad17da278e246aabe5a6a0edb7` |
| common ancestor / rc.30 | `27ff6a8767e728f879d52770c273d4f73214a430` |
| rc.31 | `36dbbf0f77e710455e745048f4a32e8120ad3fd2` |
| rc.32 | `7c044d7c5c2d2beadf16b21910950f8f593bc3ef` |
| rc.33 | `eb99ab1b40343c3317bb47981cccdbb2b159a5fa` |
| rc.34 | `0c76e4dae77a279e015329b7478e6f02d6b62edd` |
| rc.35 | `bee45b58a3c0b77e8dc81e6b5aeb4474aa9058d1` |
| rc.36 | `ea7cb0ba4e0f82e2bfa5e55752eb68bdf902f71b` |

Commands used:

```sh
git merge-base origin/develop v1.0.0-rc.36
git diff --name-only v1.0.0-rc.30 origin/develop
git diff --name-only v1.0.0-rc.30 v1.0.0-rc.36
comm -12 <(git diff --name-only v1.0.0-rc.30 origin/develop | sort) <(git diff --name-only v1.0.0-rc.30 v1.0.0-rc.36 | sort)
git log --format='%h %s' v1.0.0-rc.30..v1.0.0-rc.31
# Repeated adjacent-tag log for .31→.32→.33→.34→.35→.36.
git merge-tree v1.0.0-rc.30 origin/develop v1.0.0-rc.36
git show v1.0.0-rc.36:path/to/file.go
git diff v1.0.0-rc.30 origin/develop -- path/to/file.go
git diff v1.0.0-rc.30 v1.0.0-rc.36 -- path/to/file.go
```

`merge-tree` used its legacy three-tree, stdout-only mode. It did not change the index/worktree. Its conflict inventory is predictive; rename-aware modern merge may differ.

## Release sequence / adoption boundaries

| Release | Backend changes | Integration gate |
| --- | --- | --- |
| rc.31 | Relay hosted-tool conversion and billing-usage integrity (`0ed497f06`, `bbd97446c`); scoped log metadata (`057f71c23`, `9f506dd7f`); polling context/state/HTTP failure classification (`9df450fe5`); MiniMax-H3; no-op system-task lease fix | Largest first wave: adapt local polling and logging APIs without losing outbox/privacy logic. |
| rc.32 | Explicit `@` modifiers and canonical billing identity (`7c044d7c5`); plugin-claimed/no-channel 503; count-tokens disabled | Verify group/channel selection and model billing identity independently. |
| rc.33 | Reasoning effort preserved without implicit remapping (`6b659fd61`); expression pricing default for gpt-6-astra (`eb99ab1b4`) | Preserve operator-set pricing; test explicit effort and default expression selection. |
| rc.34 | Account verification, PAT management, separate audit logs, password/account security, Telegram OAuth; dialect migration normalization (`9a8674425`); model/vendor/pricing redesign (`0c76e4dae`) | Security frontend+backend contract must move together; biggest catalog semantic conflicts. |
| rc.35 | Plugin override-disable correctness and removal of override-layer toggle; alias-safe built-ins; hourly performance series; injectable JSON codec; Kimi dynamic-tool messages; wan3.0 | Snapshot installed custom plugin activation and aliases before changing registry semantics. |
| rc.36 | Plugin metadata/icons/default base URLs; model visibility/pricing refinements; batch redemption deletion; broad Go modernization | Validate API response shapes and channel defaulting; preserve Molii SDK/toolchain dependencies. |

## Risks and resolution policy

### Critical C1 — Terminal polling must keep Molii durable settlement

**Files:** overlapping `service/task_polling.go`, `service/task_billing.go`, `model/task.go`, `relay/relay_task.go`, `model/log.go`; Molii-only `model/task_billing_job.go`, `model/task_billing_tx.go`, `service/task_billing_reconcile.go`, `service/grok_video_billing.go`.

Molii `service/task_polling.go:115` (`finalizePolledTaskWithBilling`) creates a terminal billing intent and calls `model.FinalizeTaskAndEnqueueBillingWithContext`; `model/task_billing_job.go:74` atomically persists terminal task and job. `model/task_billing_tx.go:40` applies the fenced transaction; `service/task_billing_reconcile.go:73` reconciles durable jobs and logs.

Upstream rc.31 adds `failTaskFromPoll` whose sequence is task `UpdateWithStatus` → `settleTaskBillingOnComplete` → fallback `RefundTaskQuota`. Importing that sequence unchanged bypasses Molii's terminal+outbox atomicity. Crash gaps can strand refunds; combining direct refund with queued work can risk duplicate adjustments. Nine textual conflict regions exist in polling, but nonconflicting new helper functions are also dangerous.

**Preserve:** Molii outbox, job fencing/idempotency, transactional user/token/subscription/channel adjustments, final-only usage logs and retryable log writes, original billing snapshots, legacy cutoff behavior. **Adopt:** upstream bounded failure accounting and task/plugin state persistence, but route *every* new terminal path (404/410, threshold exhaustion, timeout, parse failure, missing channel/ID) through the existing atomic finalizer. Do not introduce a second settlement mechanism.

### Critical C2 — Polling interface break extends into untouched fork files

U36 `service/task_polling.go:27` changes:

```text
FetchTask(baseURL,key,body map,proxy) → FetchTask(baseURL,key,task *model.Task,proxy)
ParseTaskResult(body) → ParseTaskResult(task,resp *http.Response,body)
FetchBatchTasks(...,taskIDs []string,...) → (...,tasks []*model.Task,...)
ParseBatchResult(body) → ParseBatchResult(tasks,resp,body)
```

Molii-only `relay/channel/task/starai/adaptor.go:281,302` and `relay/channel/task/moliigrok/adaptor.go:759,782` still implement old signatures; their tests and injected adaptors/factories need adaptation even though these files will not produce textual conflicts. Upstream JS hooks now receive actual query contexts, per-task origin/upstream model, data/state/credentials, and response status/headers; batch hook argument two becomes task-context objects instead of ID strings. Installed non-built-in JS plugin compatibility must be checked separately from bundled source.

**Preserve:** channel-scoped task-map keys (`pollingTaskMapKey`), modern public/private ID split (do not send public IDs upstream), pinned private credentials and saved execution plugin versions. **Adopt:** full task context/state, host HTTP classification, 1 MiB persisted state/data guard. Rebuild all task adaptors and test doubles before runtime tests.

### Critical C3 — Molii polling privacy must cover new upstream failure diagnostics

Molii introduces `PrivateTaskPollingAdaptor` and `privateTaskPollingTerminalClassifier`; Molii Grok `adaptor.go:976–1009` implements safe data/error/status handling. U36 polling still has raw response/task-result debug output and new `unrecognizedPollDetail`/`recordPollFailure` warnings that may include body/error detail. Generic URL/key redaction is not equivalent to Molii's provider-specific suppression of upstream IDs, URLs, costs and credentials.

**Preserve:** provider safe output persistence, protected downloads/COS storage, sanitized errors and accepted/terminal HTTP distinctions. Ensure all new warnings, failure reasons and plugin state exposed via APIs are privacy-reviewed. Upstream now treats 404/410 as terminal; auth/transient failures count toward a default 20 failures. Explicitly reconcile this with Molii provider classifiers rather than allowing whichever branch executes first to win.

### High H1 — Security change is one cohesive frontend/backend rollout

Overlapping `controller/user.go`, `controller/oauth.go`, `controller/wechat.go`, `middleware/auth.go`, `common/session_cookie.go`, `router/api-router.go`, `model/user.go`, `model/token.go`, `controller/token.go`; mostly upstream-only `service/security_verification.go`, `service/account_security.go`, `model/account_security.go`, `model/login_verification.go`, `controller/login_verification.go`, `controller/access_token.go`, auth-flow/session helpers.

U36 `service/security_verification.go:348` consumes operation proofs once, binds user/session/action and validates live session/auth version. PAT generate/revoke uses `RequireSecurityProof` (`controller/access_token.go:24,51`). Login verification, email binding, password changes, deletion and enrollment have new contracts. Keep this complete security chain and the upstream negative tests; cherry-picking only UI endpoints weakens or breaks it.

Molii `controller/user.go` adds stable `AUTH_*` error codes and nonleaking internal errors; `model.CreateSelfRegisteredUser` and `InsertWithDefaultTokenTx` ensure registration+default API key creation is transactional (also OAuth/WeChat). Preserve them when replacing login/register sections, including new Telegram provider creation path. Keep Molii dashboard CORS validation in `common/session_cookie.go:86`, route CORS, file/video token auth, direct multi-group token selection, and key rotation.

**Password data migration:** U36 `common/account_password.go:45` defaults writes to Argon2id when `ACCOUNT_PASSWORD_HASH_ALGORITHM` is unset; existing bcrypt hashes are accepted without bulk rewrite. Rolling upgrade: deploy new dual readers everywhere with `ACCOUNT_PASSWORD_HASH_ALGORITHM=bcrypt`, then switch all nodes to argon2id and enable long passwords. Rollback must retain Argon2id verification and v2 encrypted-login envelope readers; an old binary alone is not a safe rollback after new hash writes. New-password policy is 8–128 Unicode characters; bcrypt transition mode still caps 72 bytes. Existing login validation must not suddenly reject historical passwords.

### High H2 — Audit store and typed log metadata are API + retention changes

U36 adds separate `audit_logs` (`model/audit_log.go:26,243`), no usage-log TTL/cleanup. `InitLogDB` migrates it on master even when main/log DB are shared; separate PostgreSQL/MySQL/ClickHouse stores also need the table. New login/security/management events no longer arrive solely as legacy usage log types. No historical backfill was found in the inspected migration path; do not present the new table as complete historical coverage.

U36 changes `RecordErrorLog`, consume/task billing `Other`, and helper APIs to `*model.LogOther`; audit APIs take typed `AuditOther`, actor role, optional request context. `model/log_other.go:34` has private scoped maps and `SetPublic/SetAdmin/SetRoot` setters; reserved public keys include channel ID/name/type and reject reason. Root/admin/self projections now normalize legacy fields. Molii `service/gpt_image_2_log.go:57`, Grok image log helper in `service/text_quota.go`, and `service/task_billing_reconcile.go:120` still construct/index map metadata. These untouched callers can fail compilation or leak data if mechanically wrapped as public.

**Adopt:** scoped metadata, PAT fingerprinting (trims PostgreSQL CHAR padding), actor-role visibility, token/quota operation auditing, redaction and read permissions. **Preserve:** Molii per-task final log idempotency, GPT-image/Grok preview markers and billing descriptions, user-visible safe billing data; put provider secrets and diagnostics under the correct root/admin scope. Explicit retention/storage budget required for growing audit history. Confirm `AuditRead` permission registration and Molii route audits coexist without duplicate entries.

### High H3 — PostgreSQL startup migration can merge cleanly and still regress

`model/main.go` has **no detected textual conflict**. U36 adds `postgresMigrationDialector`/`mysqlMigrationDialector` in `model/migration_dialector.go`; the PostgreSQL wrapper normalizes `bpchar` to `char` during schema comparison while retaining real length changes, and MySQL decimal-default normalization avoids pointless ALTERs. Preserve PostgreSQL `PreferSimpleProtocol: true` and disabled prepared statements for transaction-pool compatibility.

Molii migration additions must survive: `marketplaceOrderLock`, `TaskBillingJob`, `MoliiFile`; `InitializeMarketplaceDisplayOrders`, `ensureModelMarketplaceMetadataSchema`, `BackfillLocalMarketplaceMetadata`. Dropping a struct field may not drop its DB column but can silently stop reading/writing catalog/billing data. Add upstream `AuditLog` initialization for both shared and separate log DBs. Retain existing token/auth-version/session migrations.

**Test gap:** Molii `model/postgres_migration_integration_test.go:14` uses `postgres.New` directly, calls `migrateDB()` twice and checks preserved token/group rows; it does **not** exercise the new wrapped `chooseDB` or `InitLogDB` audit path. Extend coverage on a disposable PostgreSQL copy to real startup, repeat startup and SQL DDL counts, main/log split, custom catalog fields/order, pending task/job JSON, tokens and auth flows. Never point this test at production: it creates seeded rows and schema.

### High H4 — Model/vendor redesign conflicts with Molii catalog ownership

Text conflicts: `controller/model_meta.go` (12 regions), `model/model_meta.go` (6), `controller/vendor_meta.go` (3), `model/vendor_meta.go` (4), plus `model/pricing.go`, `model/option.go`. `controller/model_sync.go` is **deleted in Molii but modified upstream**; do not blindly resurrect its old upstream-sync authority.

Molii stores `DisplayName`, capabilities/modalities/parameters/resolutions/durations, `MarketplaceEnabled`, `DisplayOrder` (`model/model_meta.go:41–60`), readiness/blockers/visibility and vendor status/order. Molii local reconciliation (`model/model_catalog_reconcile.go:39`) only creates metadata for enabled channel models and preserves deleted/manual (`SyncOfficial==0`) records. Marketplace order uses its own transactional DB lock.

U36 adds metadata-optional channel listings, square states (`visible/unavailable/hidden/partial`), supported endpoints, versioned field-selective upstream preview/apply, vendor merge/move/delete previews, optional deletion from channels and pricing. `model/model_meta.go:261–268` updates an explicit upstream field whitelist: it would omit Molii catalog fields if accepted unchanged. Vendor updates deliberately preserve saved status, which conflicts with Molii's vendor enable/disable controls. Upstream `metadataTransaction` (`model/model_metadata_sync.go:19`) locks `options.metadata_sync_lock`; Molii uses `marketplaceOrderLock`.

**Preserve:** local catalog metadata/publication intent/status/order, authorization and response fields used by Molii pricing/marketplace. **Adopt selectively:** safe reference checks, optimistic versions, transactional channel+abilities updates and after-commit pricing refresh. Decide how upstream remote synchronization fits local-only catalog ownership before exposing its routes. Define one consistent lock acquisition order across metadata/pricing/marketplace locks to avoid cross-node deadlocks. Model delete with `remove_pricing=true` must include Molii custom price maps or explicitly reject that option for those models; otherwise stale/custom prices survive deceptively. Deletion from channels must remain exact-name only.

### High H5 — Routing and billing identities must remain distinct

rc.32 introduces explicit `@` modifiers and `BillingModelName`; U36 `relay/helper/price.go:75–88` selects pricing identity, `RelayInfo.GetBillingModelName` leaves client/routing names distinct, `ModelMappedHelper` can fall back to base model mappings. rc.33 intentionally removes implicit effort remapping. Molii `relay/helper/model_mapped.go:22–79` handles responses compact suffix by stripping for routing then rewriting `OriginModelName` to a mapped compact billing key, and validates incompatible Imagine-family mappings. Those semantics overlap.

**Adopt:** canonical billing identity and explicit reasoning modifiers; move compact price alias behavior into billing identity without contaminating channel/model selection or user-visible original identity. **Preserve:** Imagine-family validation, cyclic-map errors, image edit/stream selection, direct-group and group-order rules, StarAI/Grok channel registration and billing policies. Test exact aliases, chained mappings, @effort, passthrough, compact aliases, group model limits, retries and unmapped requests. No blanket replacement of Molii ratio/default maps with upstream defaults.

### High H6 — Relay conversion and billable usage need cross-provider fixtures

Upstream changes live across `relaykit/dto`, `relaykit/relayconvert`, relay handlers and usage normalization, even though most are not textual conflicts. Changes include hosted-tool loss policy/diagnostics, reasoning normalization, independent native billing usage, streamed conversion state resets on retries, model-specific chat capability filtering, and Kimi dynamic tool loading. Molii overlaps `relay/common/relay_info.go`, `tool_usage.go`, OpenAI image/responses handlers, `relaykit/dto/openai_response.go`, `service/text_quota.go`.

**Adopt:** native upstream billing usage integrity, explicit capability/reasoning conversion and retry state reset. **Preserve:** Molii x-search/code/attachment/collections tool counting; reject failed/cancelled/incomplete/partial tool outputs for charging; Grok image zero-cost snapshots, final response completion timestamp, GPT-image persistence and preview, custom pricing snapshots. A textual auto-merge of `tool_usage.go`/`text_quota.go` is not sufficient proof of correct tool charge totals. Test Chat/Responses/Claude/Gemini stream+nonstream native usage, cache creation/hits, hosted tools, retries and cancellations. `relaykit` is a nested module; root `go test ./...` does not validate its whole independent suite.

### Medium M1 — Plugin deployment and channel defaults change

rc.35 removes `TASK_PLUGIN_OVERRIDE_ENABLED` and registry `SetOverrideEnabled`; `TASK_PLUGIN_ENABLED` remains the master switch. A deployment that intentionally set only override=false will no longer have that behavior. Disabling an overridden plugin now suppresses its factory fallback; verify current installed override states, versions, source checksums and channel bindings before rollout. U36 alias-safe built-ins should be adopted but test public alias echo for Molii plugin channels.

rc.36 adds plugin sort priority, website, baseUrl, icons, usage enum labels. `controller/channel.go` now permits plugin default base URL to fill an empty base URL and persists it. Preserve Molii private-provider base URL validation, blank-key update semantics and channel-type binding checks. Defaulting cannot bypass protected outbound URL policy.

### Medium M2 — Performance response contract and counters

U36 replaces `recent_success_rates: number[]` with `recent_success_series: {timestamp,success_rate}[]` (`pkg/perf_metrics/types.go`) and aggregates hourly points. Molii modifies counters/window calculations and has group success-rate rankings, request counts and video performance records. Merge both data models; update Molii consumers/fixtures or temporarily emit both fields. Keep request/attempt and terminal success/failure semantics unchanged for rankings. Conflicts exist in `pkg/perf_metrics/metrics.go` (2 regions) and `types.go` (1).

### Medium M3 — Environment, dependencies and mixed-version operation

New behavior: `TASK_POLL_MAX_FAILURES=20` (<=0 disables count cutoff), documented `TASK_TIMEOUT_MINUTES=1440` (0 disables age cutoff); removed override switch; Argon2id rollout switch above. Preserve all Molii `DASHBOARD_CORS_ALLOWED_ORIGINS`, storage/COS/StarAI, maintenance, deployment and shared session-secret settings. Review templates without printing actual secrets.

Molii root Go module requires Go 1.26.5 and newer x/image, x/sync, x/text; keep that floor. U36 adds OIDC/oauth2, bumps ClickHouse driver 2.32→2.46, SQLite 1.9→1.11, compression libraries. Preserve Molii `github.com/tencentyun/cos-go-sdk-v5` and its indirect dependencies. Resolve `go.mod`/`go.sum` as a union and verify root/nested modules and deployment images. Broad rc.36 `any`, `SplitSeq`, `reflect.Pointer` modernization is mostly mechanical; do not misinterpret it as substantive Redis storage-format migration. Inspected Redis/auth-fence changes retain existing cache keys and mostly modernize syntax; new account mutation/session creation still needs revocation race/outage tests.

### Low L1 — Straightforward fixes, still regression-test

Adopt system-task no-op lease check (`b7017c251`; `model/system_task.go:322`) while retaining Molii context/error propagation. Upstream ETag and injectable JSON codec fixes should move with relaykit/common tests; retain Molii HTTP/CORS wrappers. Batch redemption delete needs its permission/size/invalid-ID tests but has little custom data overlap.

## API contract checklist

- Login new `/api/user/login/verify`, `/login/passkey/begin`, `/login/passkey/finish`; verification methods `/api/verify/methods`; operation-bound proof consumption. Existing `/login/2fa` compatibility must be tested.
- `/api/user/token` GET compatibility handler plus POST generation, DELETE revocation, `/status`; generation/revocation now require scoped security proof. Distinguish this dashboard PAT from `/api/token` relay API keys and preserve Molii API-key rotation.
- Email bind start/resend/complete contracts changed. Telegram goes through unified OAuth; legacy login/bind routes dispatch `TelegramLegacyAuth`, not the previous widget flow. Require deployment configuration/UI coordination, not silent route replacement.
- `/api/audit` (admin+AuditRead), `/api/audit/self`; new events move from usage log views to audit views, with role-scoped details and retention independent of usage logs.
- `/api/option/model_pricing` GET/PATCH, versioned model sync, vendor operation preview/apply, model batch delete and optional channel/pricing deletion. Coordinate Molii custom field preservation and routes before adoption.
- Task plugin icon endpoint and metadata/base URL fields; JS polling hook arguments change as above.
- `/messages/count_tokens`: upstream explicitly disables it; router already lacked an active count route at rc.30. Do not advertise a newly supported route, and do not accidentally re-enable via a conflict resolution.
- Performance summary replaces recent-rate array with timestamped hourly series.

## Files Found / overlap inventory

All overlapping non-web implementation/config paths (test-only paths omitted here; full command above reproduces them):

```text
.env.example
common/init.go common/session_cookie.go
constant/env.go constant/task.go
controller/channel-test.go controller/channel.go controller/channel_upstream_update.go
controller/group.go controller/log.go controller/model.go controller/model_meta.go
controller/model_sync.go controller/oauth.go controller/option.go controller/relay.go
controller/token.go controller/user.go controller/vendor_meta.go controller/wechat.go
go.mod go.sum
middleware/audit.go middleware/auth.go middleware/distributor.go middleware/rate-limit.go
model/channel.go model/channel_cache.go model/log.go model/main.go model/model_meta.go
model/option.go model/perf_metric.go model/pricing.go model/pricing_default.go
model/subscription.go model/system_task.go model/task.go model/token.go model/user.go model/vendor_meta.go
pkg/perf_metrics/metrics.go pkg/perf_metrics/types.go
relay/channel/adapter.go relay/channel/openai/relay_image.go relay/channel/openai/relay_responses.go
relay/channel/task/jsplugin/adaptor.go relay/common/relay_info.go relay/common/relay_utils.go
relay/common/tool_usage.go relay/helper/model_mapped.go relay/relay_task.go
relaykit/dto/openai_image.go relaykit/dto/openai_response.go
router/api-router.go router/main.go router/relay-router.go
service/quota.go service/rankings.go service/task_billing.go service/task_polling.go service/text_quota.go
setting/billing_setting/tiered_billing.go setting/operation_setting/tools.go setting/ratio_setting/model_ratio.go
```

Predicted textual conflicts from legacy merge-tree (number = marker regions):

```text
1 .env.example                         1 common/init.go
1 controller/billing_option_test.go    1 controller/channel.go
2 controller/group.go                12 controller/model_meta.go
1 controller/relay.go                 2 controller/token_test.go
1 controller/user.go                  3 controller/vendor_meta.go
2 go.mod                              1 middleware/audit.go
1 middleware/distributor_test.go      2 model/log.go
6 model/model_meta.go                 1 model/option.go
1 model/pricing.go                    1 model/task.go
4 model/vendor_meta.go                2 pkg/perf_metrics/metrics.go
1 pkg/perf_metrics/types.go           2 relay/common/relay_info.go
1 relay/helper/model_mapped.go        6 router/api-router.go
1 router/main.go                      3 service/task_billing.go
9 service/task_polling.go             2 service/task_polling_test.go
1 setting/ratio_setting/model_ratio.go
modify/delete: controller/model_sync.go
```

Do not equate this list with required work: fork-only adaptors/loggers and automatically merged security/migration files are major semantic hotspots.

## Dependencies / existing patterns to preserve

```text
router → middleware credential/group/proof policy → controller
  → model auth-flow/session transaction + Redis deny/version fences
  → scoped audit log writer → LOG_DB (independent retention)

relay request → original model / group selection → channel mapping
  → canonical billing model + frozen pricing snapshot
  → provider adapter / relaykit conversion → native BillingUsage → quota/logging

submit → task+saved execution/billing context → background polling
  → safe provider data + failure/state accounting
  → terminal task + durable billing job in ONE main-DB transaction
  → fenced reconciliation → quota/token/subscription/channel tx
  → idempotent billing log → separately protected result storage/preview

catalog controllers → metadata/marketplace/pricing locks (consistent order)
  → model/vendor/custom-field transaction → abilities update → post-commit caches
```

## Staged tests and deployment gates

1. **Static/interface gate:** resolve source contracts, build root and `relaykit` modules; inspect all implementations of polling interfaces and all log writers. Preserve complete Molii tests. No successful compilation can be claimed from this audit.
2. **Fast package suites:** `go test ./common ./model ./middleware ./controller ./router ./service ./pkg/jsplugin ./pkg/perf_metrics ./relay/... ./setting/...`; run nested `go test ./...` with workdir `relaykit`. Run root `go test ./...` afterwards and race tests for auth/metadata/polling where practical. Use existing Docker/toolchain convention if required by workspace.
3. **Disposable DB gate:** set `FULL_MIGRATION_POSTGRES_TEST_DSN` to a fresh copied test database and run `go test ./model -run TestPostgresFullSchemaMigrationIsIdempotent -count=1`; augment with actual `chooseDB` + `InitLogDB` startup twice, split log DB, no redundant ALTERs, representative pending tasks/plugin state/jobs/catalog/order. Existing test intentionally skips without DSN, so a green skipped suite is not PostgreSQL evidence. Exercise U36 migration-dialector tests; MySQL checks if deployed, ClickHouse audit JSON/driver if configured.
4. **Disposable Redis/security gate:** `TEST_REDIS_DSN=... go test ./common -run TestConfiguredRedisRoundTripPreservesTTL -count=1`; then session/proof integration coverage for DB/Redis failures, revoke races, refresh reuse, operation replay, wrong action/user/session, stale auth version, PAT vs session, logout/CORS. Test existing bcrypt and new Argon2id with mixed-reader deployment policy; registration defaults exactly once for password/OAuth/WeChat/Telegram. Audit must contain no bearer token/raw body/URL query and respect common/admin/root views.
5. **Financial fault-injection gate:** test duplicate poll workers, crash after terminal+job commit, crash after quota commit before log write, log DB outage/retry, expired subscription, token removed/rotated, missing private upstream ID, same upstream ID on two channels, timeout vs success race, 19/20 poll failures then recovery, 404/410 vs 401/403/429/5xx, missing batch item and malformed/oversized plugin state. Assert one terminal transition/job, one net settlement, one final log, preserved user/token/channel totals and no secret leakage.
6. **Relay/provider gate:** Molii StarAI/Seedance, Grok image/generation/edit/extension/video, GPT-image-2 stream/nonstream persistence, protected files/results, plugin alias echo and disable/factory suppression, private key pinning. Cross-format Chat/Responses/Claude/Gemini stream+nonstream tool/usage/reasoning fixtures; @model aliases, compact mapping and retry resets; zero-priced and per-call adjustment cases.
7. **Catalog/API gate:** snapshot custom metadata and prices before create/update/delete/sync/vendor move/merge; assert no omitted custom fields, changed publication, reordered display entries or vendor re-enablement. Test exact channel removals, pricing-map completeness, stale preview rejection, concurrent order vs metadata mutations. Verify Molii frontend consumes audit/security/performance changes and stable AUTH_* errors.
8. **Canary/rollback gate:** backup main+log DB and export nonsecret option/schema/plugin state; drain or explicitly account for old workers/pending tasks before switching settlement behavior. Keep new verification readers across rollback once Argon2id/v2 writes begin. Canary startup DDL/auth/task settlement metrics; compare billed amounts and visibility against fixed fixtures before wider rollout. Do not run credentialed provider submissions or destructive catalog operations without separate authorization.
