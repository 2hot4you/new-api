# Web Dev Catalog Managed Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 各环境超级管理员在 Web 拉取 dev 的供应商、模型元数据和定价，预览字段级 changelog，确认后安全受管同步并可恢复。

**Architecture:** 固定 dev 只读来源，由目标后端拉取完整快照并保存有期限的计划。纯契约与三方差异算法放在无数据库依赖的包，model 层持有事务、基线、版本、备份及运行时发布屏障，service/controller 负责网络和会话授权。复用 CLI 的记录映射，不复用其 PostgreSQL 限定 apply 或整块覆盖语义。

**Tech Stack:** Go 1.25.1、Gin、GORM、SQLite/MySQL/PostgreSQL、React 19、TypeScript、TanStack Query/Table、现有安全验证与 Base UI 组件、Bun/Vitest。

**Spec:** `.ccg/tasks/web-dev-catalog-sync/design.md`（用户已批准）。

## Global Constraints

- 只从 https://dev.molii.co 拉取；15 秒超时；解压后响应上限 10 MiB；服务端计划有效期 10 分钟。
- A 受管同步：目标独有数据保留；仅可信完整来源移除的已受管项可以提出删除；删除需单独确认，危险回退或仍被引用时阻断。
- 同步供应商、持久化模型资料、计费币种、原始表达式、插件覆盖和明确的专项价格；不改渠道、用户、余额、密钥、品牌、汇率、充值或分组。
- 本地修改冲突必须明确覆盖；首版整批应用，不提供逐字段部分应用。
- 首版只支持每站一个应用实例；检测多实例/未知部署形态时禁止应用。部署不等于授权首次同步生产数据。
- 业务 JSON 使用 common 包装；不新增生产依赖；不建立第二套计费规则；不自动转换传统定价或凭空补造元数据。
- 支持真实 SQLite、MySQL、PostgreSQL；禁止用单一数据库或跳过的集成测试声称完成。
- 用户要求不使用 antigravity/Claude 外部执行器。实施前若其他适用指令要求调用这些执行器，必须明确处理冲突，不得静默调用或声称已经双模型审查。
- 计划/设计放在当前 CCG 任务目录，不新建 docs 文件；保护现有未跟踪的 ` 2.ts`/` 2.tsx` 文件；只提交精确列出的任务变更。

## Review Focus

1. 删除一个嵌套工具价格不能删除目标独有的同级叶子项；Task 2 的 nested-leaf preservation 用例保护。
2. 源导出期间管理员改价不能生成跨时刻混合快照；Task 3 的 source-concurrent-mutation 用例保护。
3. 同步与常规定价/元数据保存竞争不能丢失更新或观察半套币种；Task 4 的 concurrent-normal-save 和 atomic-currency-price 用例保护。
4. 数据库提交后进程崩溃或响应丢失不能重复应用、假报失败或使用旧价继续计费；Task 5 的 committed-before-publish-restart 和 response-lost-retry 用例保护。
5. 软删除后再次新增同名对象不能让旧基线误认新对象为可删除；Task 2/4 的 recreated-same-name 用例保护。

## File and Ownership Map

- `pkg/catalogmanifest/{types.go,diff.go}`：无 model/service 依赖的 v2 契约、规范化、摘要与三方合并；避免 internal/catalogsync 已依赖 model 导致反向导入环。
- `internal/catalogsync/{snapshot.go,plan.go,catalogsync_test.go}`：保持 v1 CLI 接口及原行为；新增 Web v2 导出桥接和覆盖检查，不改变 CLI apply 默认策略。
- `model/catalog_sync{,_store,_apply,_runtime}.go`：持久化记录、跨库事务、应用/恢复、发布和恢复门禁；新增集中 `model/catalog_sync_test.go` 承载数据库契约测试。
- `model/{main.go,model_pricing_config.go,model_meta.go,vendor_meta.go,vendor_management.go,model_metadata_sync.go,model_catalog_reconcile.go,option.go}`：迁移登记与现有写入路径协调；`model/marketplace_display_order.go` 的排序写入也须协调。
- `model/{model_billing_money.go,pricing.go,pricing_refresh.go}`、`relay/helper/{price.go,billing_expr_request.go}`、`service/task_billing.go`、`main.go`：价格快照发布/新请求门禁、任务快照及启动恢复，不能持锁等待上游或流式输出结束。
- `service/catalog_sync.go`、`controller/catalog_sync.go`、`router/api-router.go`：拉取、管理 API、只读导出认证；`service/security_verification.go` 注册绑定摘要的新 scope，沿用现有验证框架。
- `web/src/features/system-settings/catalog-sync/{api.ts,types.ts,index.tsx,changelog.tsx,confirm-sync-dialog.tsx,history.tsx,__tests__/catalog-sync.test.tsx}`：只承载同步业务；复用共享表格、ConfirmDialog、安全验证、错误/加载状态。
- `web/src/features/system-settings/billing/{molii-aigc-pricing-tabs.tsx,section-registry.tsx}`：增加环境同步标签，保留已有通用模型/Seedance/Grok 及上游同步入口；`web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json` 加文案。
- `deploy/env/{development,production-molii,production-ixiaozu,production-claudeye,production-model-claudeye}.env.example`：非秘密配置说明；`deploy/tests/deploy_test.sh` 验证 runtime 文件保留；`.github/workflows/deploy.yml` 增加三数据库验收门禁。

## Shared Interfaces

`pkg/catalogmanifest` 定义以下公开类型，后续任务不能自行改名：

- `Actor{UserID int; SessionID string; TargetID string}`，由现有会话身份显式映射，不保存会话 token。
- `Entry{Kind string; Key string; Value string}`：Kind 为 vendor/model/model_price/plugin_price/special_price/tool_price；Value 为规范 JSON，Key 不含数据库 ID；实际叶子路径用 JSON Pointer 而非点号拼接，防止含点模型名冲突。
- `Snapshot{SchemaVersion int; SourceID string; ExportedAt int64; Digest string; Complete bool; Capabilities map[string]string; Entries []Entry}`：版本 2；价格类别 Key 必须结构化编码，区分 plugin key 与模型名。
- `Baseline{SourceID string; Generation int64; Entries []Entry; ObjectVersions map[string]string}`：Value 是上次成功应用值；ObjectVersions 识别目标记录代次，不把软删除后新记录自动当成原对象。
- `Resolution{OverwriteKeys []string; ConfirmDeletes bool}`；`Change{Kind,Key,Action,Reason string; Before,Base,After *Entry}`；Action 为 create/update/delete/adopt/conflict/preserve/blocked。
- `Plan{ID string; Snapshot Snapshot; TargetDigest string; BaselineGeneration int64; Actor Actor; ExpiresAt int64; Resolution Resolution; Digest string; Changes []Change}`。
- `Result{OperationID,State string; Revision int64}`：State 为 committed_pending_publish/succeeded/failed；失败错误不泄露凭证。

所有摘要均基于规范化内容，忽略展示时间但包含来源、目标、覆盖范围、兼容版本、所需引用与明确确认的选择。外部请求不直接反序列化成可执行 Plan。

### Task 1: 固化 v2 快照契约和完整导出

**Files:** 新建 `pkg/catalogmanifest/types.go`；修改 `internal/catalogsync/snapshot.go`；扩展现有 `internal/catalogsync/catalogsync_test.go`。

**Interfaces:** `catalogmanifest.ValidateSnapshot(Snapshot) error`；`catalogmanifest.SnapshotDigest(Snapshot) (string,error)`；`catalogsync.ExportManaged(ctx context.Context, tx *gorm.DB, sourceID string) (catalogmanifest.Snapshot,error)`。只读事务及来源写入屏障由 Task 3 调用方持有。

- [ ] 写失败用例 `TestManagedExportCompletePricingContract`：独立插件价格不当模型名；无元数据显式价格不漏导出；无法确定币种阻断；显式 0/缺失不同；未知价格字段阻断；一般 options 和凭证不出现。
- [ ] 运行 `go test ./internal/catalogsync -run TestManagedExport -count=1`，配置其 PostgreSQL 测试 DSN；确认失败不是依赖/数据库缺失，SKIP 不算红灯。
- [ ] 实现稳定键、v2 覆盖声明、字段白名单及持久化元数据完整映射。专项白名单从当前 settings 的实际价格字段定义生成/复用，不用宽泛前缀。内置默认值不生成显式价格。
- [ ] 同命令转绿，再运行现有 CLI 测试确认 v1 快照和“不删除目标模型”契约未变；提交精确文件，提交信息 `feat: add complete managed catalog manifests`。

### Task 2: 受管三方差异与字段级 changelog

**Files:** 新建 `pkg/catalogmanifest/diff.go`、`pkg/catalogmanifest/diff_test.go`。

**Interfaces:** `BuildPlan(source Snapshot, target Snapshot, base Baseline, actor Actor, now time.Time) (Plan,error)`；`ResolvePlan(plan Plan, choices Resolution) (Plan,error)`；引用检查结果以 blocked Change 合并，禁止前端取消阻断。

- [ ] 表驱动失败用例 `TestManagedThreeWayDiff`：首次相同值产生 adopt；目标独有 preserve；本地修改冲突；嵌套独有叶子保留；只删除基线已管理项；空/部分源或 source_id 变更不能删；同名重建产生冲突；币种与表达式同一模型确认单元。
- [ ] 运行 `go test ./pkg/catalogmanifest -count=1`，确认红灯。
- [ ] 实现按 Kind+结构化 Key 排序的规范化比较，完整 B/T/S 规则；选择覆盖或删除确认后重算摘要，不改变固定源快照；无选择的冲突不可执行。
- [ ] 运行同命令转绿；提交 `feat: plan managed catalog changes without pruning local entries`。

### Task 3: 跨库计划/基线存储和稳定来源快照

**Files:** 新建 `model/catalog_sync.go`、`model/catalog_sync_store.go`、`model/catalog_sync_test.go`；修改 `model/main.go`、现有元数据/价格写入路径（见 ownership map）。

**Interfaces:** `model.CreateCatalogSyncPlan(ctx context.Context, source catalogmanifest.Snapshot, actor catalogmanifest.Actor, now time.Time) (catalogmanifest.Plan,error)`；`model.GetCatalogSyncPlan(ctx context.Context, id string, actor catalogmanifest.Actor) (catalogmanifest.Plan,error)`；`model.WithCatalogReadSnapshot(ctx context.Context, read func(*gorm.DB) error) error`。

- [ ] 写 `TestCatalogSyncStore` 和 `TestCatalogSyncSourceConcurrentMutation`：10 分钟期限、会话/目标绑定、全同值接管、完整来源快照、防软删除后身份混淆、升级后两次迁移无额外 schema 变化。
- [ ] 三库运行 `go test ./model -run 'TestCatalogSync(Store|SourceConcurrentMutation)' -count=1 -v` 确认红灯；测试使用既有 `TEST_MYSQL_DSN`/`TEST_POSTGRES_DSN` 和真实 SQLite。测试 DSN 必须指向临时测试库。
- [ ] 建立 `CatalogSyncState`（固定主键版本/发布状态）、`CatalogSyncPlan`（固定快照、选择、会话绑定、期限）、`CatalogSyncBaseline`（受管条目、对象代次）、`CatalogSyncOperation`（备份、历史、幂等结果）表。大正文用兼容 TEXT/长文本策略；复合身份索引用规范 Key 的固定 64 字符摘要，避免 MySQL 旧版本索引过长。
- [ ] 所有目录/价格写入协调同一 model 层锁与 revision；SQLite 使用明确串行写入、MySQL/PostgreSQL 使用共享 model 锁方法。固定状态行先初始化再锁，不能锁不存在行。标准保存、排序、官方资料同步、渠道导致的目录自动补齐都必须纳入；源导出在此屏障和只读事务内获得一致快照。
- [ ] 同命令三库转绿，再回归原模型/供应商写入测试；提交 `feat: persist managed sync plans and ownership baselines`。

### Task 4: 有保护的事务应用与最近一次恢复

**Files:** 新建 `model/catalog_sync_apply.go`；扩展 `model/catalog_sync_test.go`；按 Task 3 接口扩展存储文件，复用既有元数据和价格校验，不调用会提前发布运行时状态的普通保存函数。

**Interfaces:** `model.ApplyCatalogSyncPlan(ctx context.Context, planID, finalDigest, operationID string, actor catalogmanifest.Actor) (catalogmanifest.Result,error)`；`model.CreateCatalogSyncRestorePlan(ctx context.Context, operationID string, actor catalogmanifest.Actor, now time.Time) (catalogmanifest.Plan,error)`；`model.ListCatalogSyncOperations(ctx context.Context, offset,limit int) ([]CatalogSyncOperation,int64,error)`。

- [ ] 写 `TestCatalogSyncApplyAndRestore`：备份写入失败全回滚；普通改价竞争返回冲突；确认后 dev 更新不影响固定快照；同操作重复提交无重复；价格与币种原样；仍被渠道/映射/未完成任务引用阻止删除；供应商被独有模型引用阻止删除；恢复不覆盖后续本地修改；同名重建不误删。
- [ ] 三库运行 `go test ./model -run 'TestCatalogSync(ApplyAndRestore|Store)' -count=1 -v` 确认红灯。
- [ ] 在统一锁内重验计划/对象代次/revision/全部引用，验证目标现有插件绑定和完整价格草稿；按供应商→模型→价格→安全删除→基线顺序写入，并同事务保存前值备份和操作 ID。无法解释的渠道匹配、专项回退或插件 schema 直接 blocked，不猜测兼容性。
- [ ] 按设计只生成最近一次成功操作的逆向计划，复用相同确认和应用流程；恢复后还原此次受管基线但不回退全站 revision。新建对象的恢复删除也执行引用保护。备份和历史不含完整站点 options。
- [ ] 三库转绿；提交 `feat: apply and restore managed catalog transactions safely`。

### Task 5: 运行时批量发布、计费快照与崩溃恢复

**Files:** 新建 `model/catalog_sync_runtime.go`；修改 `model/model_pricing_config.go`、`model/option.go`、`model/model_billing_money.go`、`model/pricing.go`、`model/pricing_refresh.go`、`relay/helper/price.go`、`relay/helper/billing_expr_request.go`、`service/task_billing.go`、`main.go`；扩展 `model/catalog_sync_test.go`，沿用已有 relay/task 测试文件补关键快照回归。

**Interfaces:** `model.PublishCatalogSyncRevision(ctx context.Context, revision int64) error`；`model.RecoverCatalogSyncRuntime(ctx context.Context) error`；`model.WithCatalogPricingRead(ctx context.Context, modelName string, capture func() error) error`。Task 4 在提交后调用发布，只有发布成功才将操作标记 succeeded。

- [ ] 写 `TestCatalogSyncAtomicCurrencyPrice`、`TestCatalogSyncCommittedBeforePublishRestart`、`TestCatalogSyncResponseLostRetry`：不能观察到新币种/旧表达式；删除后内存无旧条目；提交后崩溃启动恢复；重复操作查询原结果；发布失败只阻断受影响新请求；在途任务继续旧快照。
- [ ] 运行 `go test ./model ./relay/helper ./service -run 'TestCatalogSync|Test.*BillingSnapshot' -count=1`，确认红灯。
- [ ] 发布前构建并校验整批运行时配置；常规保存和后台 SyncOptions 加入同一屏障。读屏障仅包住解析及冻结价格快照，不跨网络、流或任务生命周期持锁。同步发布重建删除项默认状态并失效公开定价/元数据缓存。
- [ ] committed_pending_publish 持久化并在启动初始化选项后、接收请求前恢复；发布失败不得报成功。使用当前 SystemInstance 心跳和明确单实例配置双重检查，无法验证单实例则关闭 apply；不声称已支持多节点一致性。
- [ ] 相关测试转绿并运行 `go test -race ./model ./relay/helper ./service`；提交 `feat: publish synchronized pricing snapshots without restarts`。

### Task 6: 来源只读认证、网络限制和超级管理员 API

**Files:** 新建 `service/catalog_sync.go`、`controller/catalog_sync.go`、`controller/catalog_sync_test.go`；修改 `service/security_verification.go`、`router/api-router.go`；扩展既有 `controller/security_enrollment_test.go` 验证 scope；配置在新 service 以受控环境读取，不放普通 options。

**Interfaces:** `service.FetchDevCatalog(ctx context.Context) (catalogmanifest.Snapshot,error)`；管理根路径 `/api/catalog_sync`：`GET /status`、`POST /preview`、`POST /plans/:id/resolve`、`POST /plans/:id/apply`、`GET /history`、`GET /operations/:id`、`POST /operations/:id/restore-preview`。只读来源为 `GET /api/catalog_sync/export`，独立专用凭证认证，不走管理登录认证。

- [ ] 实施前完整阅读项目要求的 OWASP Authentication、Session Management、CSRF 指南及相关当前 ASVS 控制，将引用和未满足项记入本任务 review.md，不声称未经验证的合规。
- [ ] 写 `TestCatalogSyncHTTP`：普通管理员/个人 token 不可应用、错误/撤销来源凭证拒绝、token 不出日志/响应、私网/DNS 重绑定/重定向拒绝、15 秒超时和解压后 10 MiB 限制、CSRF、会话过期、10 分钟计划过期、安全证明错误 scope/context/过期/重放、只读导出不能写其他接口。
- [ ] 运行 `go test ./controller -run 'TestCatalogSyncHTTP|Test.*Security' -count=1`，确认红灯。
- [ ] 固定 HTTPS dev 地址及受限 DialContext，校验解压后大小；源以独立凭证哈希列表认证并限流，不降级公开 pricing。注册 `catalog.sync.apply` 与 `catalog.sync.restore` scope，绑定计划最终摘要和目标；沿用 RequireSecurityProof，不复制渠道读取 scope。
- [ ] 环境契约：`CATALOG_SYNC_ROLE=source|target|disabled`（默认 disabled）、`CATALOG_SYNC_SOURCE_ID`（稳定 dev 身份）、目标 `CATALOG_SYNC_TARGET_ID`、目标秘密 `CATALOG_SYNC_TOKEN`、源 `CATALOG_SYNC_READERS_JSON`（凭证 ID→哈希及状态）、`CATALOG_SYNC_SINGLE_INSTANCE=true`。只读 status 返回角色/就绪状态/非秘密标识；v1 凭证通过受保护 runtime 环境配置及轮换，不新增 Web 明文密钥保存流程。
- [ ] resolve 保存冲突覆盖/删除确认并生成最终摘要，apply 请求只接收 plan ID、摘要、operation ID 和安全证明，不接收 Snapshot。HTTP 409 用于版本冲突，422 用于阻断，503 用于来源/发布不可用；超时客户端查询 operation ID，不自动重放安全证明。
- [ ] 安全/API 测试转绿；提交 `feat: expose authenticated managed catalog synchronization APIs`。

### Task 7: 超级管理员 Web 同步、确认和历史

**Files:** ownership map 中前端新 feature、标签集成与七种 locale；扩展已有 `billing/__tests__/molii-aigc-pricing-tabs.test.tsx`。

**Interfaces:** `CatalogSyncSection(): React.JSX.Element`；api.ts 导出 `getCatalogSyncStatus`、`previewCatalogSync`、`resolveCatalogSyncPlan`、`applyCatalogSyncPlan`、`listCatalogSyncHistory`、`getCatalogSyncOperation`、`previewCatalogSyncRestore`，类型逐项对应 Task 6 服务端响应。

- [ ] 使用项目 shadcn-ui 技能及组件复用规则：已确认 ConfirmDialog 的 children 可以承载删除勾选与摘要，安全验证使用现有 use-secure-verification，表格从 data-table 公开入口导入；不得另写确认/安全验证通用组件。新增 feature 组件只承载业务。
- [ ] 写 `catalog-sync/__tests__/catalog-sync.test.tsx`：非超级管理员无入口、来源只显示状态、不自动同步、六类 changelog/字段与表达式长文本、币种和单位、冲突选择、独有项保留、删除未勾选不可应用、阻断不可绕过、无变化不可应用/仅接管可确认、过期重新预览、操作 pending 不重复提交、断线查原结果、恢复需重验、键盘/焦点和语言切换。
- [ ] 运行 `bun run test src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx src/features/system-settings/billing/__tests__/molii-aigc-pricing-tabs.test.tsx`（web 目录）确认红灯。
- [ ] 新增第四个“环境同步”标签，仅 root 渲染且后端同样鉴权。React Query 管理服务端状态；选项通过 resolve 保存后再申请绑定最终摘要的安全证明。显示目标站及时间版本；历史长列表分页，每页 20，最大 100；快照文本按纯文本渲染，不使用 HTML 注入。
- [ ] 同测试转绿，运行 `bun run typecheck`、受影响文件 lint、`bun run i18n:check`、`bun run build`；提交 `feat: add managed catalog sync changelog and confirmation UI`。

### Task 8: 集成矩阵、代码审查与交付

**Files:** 扩展 `.github/workflows/deploy.yml`、`deploy/tests/deploy_test.sh`、五份 deploy/env 示例；任务目录 `review.md` 和 `task.json`。不得写任何真实凭证进入仓库。

- [ ] 配置临时 SQLite/MySQL/PostgreSQL 测试实例和最小权限测试 DSN；三库运行全部 TestCatalogSync 测试。fresh/旧版代表性库升级均跑两次迁移并检查原数据/索引；此验收不得复用生产库。CI 配置测试预先验证缺 DSN 会失败，不能默默 SKIP。
- [ ] 运行 `go test ./...`、受影响包 race、`go build ./...`，web 完整 test/typecheck/受影响 lint/i18n/build；部署脚本 `bash deploy/tests/deploy_test.sh`，确认原 runtime secrets 不被部署覆盖。若最低数据库版本受实现差异影响，增加最低版本实测，不用新版本结果替代。
- [ ] 审查权限绕过、SSRF、来源完整性、所有权删除、并发写入、缓存发布与恢复，记录真实命令、数据库版本、失败/修复和未解决问题到 review.md；出现指令冲突或无法完成三库/安全验证时明确阻断，不伪称通过。
- [ ] 部署配置默认 disabled，dev 配只读源、各目标单独配置凭证，预览导出不得带秘密。设计代码提交并不授权 push、生产发布或首次目录 apply；获得相应用户指令后才执行。
- [ ] 发布时先 dev 再目标；仅验证连接、只读预览、权限和健康，不付费调用上游，不自动 apply。目标具体域名在部署前确认，无需为写功能猜测三站名单。
- [ ] 最后核对精确 git diff、无秘密、无用户 ` 2` 文件，归档本 CCG 任务并提交。只有实现、审查、必要验证与获授权的交付均完成才标记 completed。

## 执行依赖和评审结果

顺序：1 → 2 → 3 → 4 → 5 → 6 → 7 → 8。网络/UI 可以在契约冻结后用 native Codex 子代理按文件分工准备，但主干 model 事务/发布文件必须单人串行，controller 与 service 的相互依赖不可各自猜测接口。不得省略最终整条链路验证。

自审：设计 1–3 由 Task 1/6/7 覆盖；设计 4 由 Task 2/4 覆盖；设计 5–6 由 Task 6/7 覆盖；设计 7 由 Task 3/4 覆盖；设计 8 由 Task 5/4 覆盖；设计 9 的数据库和发布边界由 Task 8 覆盖。五项 Review Focus 均已指定回归测试；shared interfaces 使用同一包，避免 model→internal 导入环。尚未运行上述产品测试：它们是实施验收命令，不是通过记录。

当前仅计划编写完毕，等待用户评审并选择执行方式；没有实施业务代码或线上写入。
