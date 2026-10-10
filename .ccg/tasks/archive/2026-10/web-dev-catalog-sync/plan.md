# Web Dev Catalog Managed Sync Implementation Plan

完成更新（2026-10-10）：Tasks1–8及整功能最终审查已完成，单次修复 c9e1416f7 经原审查员复审通过。详细阶段索引见 execution-ledger.md；原始勾选项及旧范围文字保留为计划历史，不构成重新执行其他数据库的授权。PostgreSQL基线全门禁和最终修复专项证据分别记录，不宣称重新全量验证或线上已发布。仅本地提交，分支/工作树保留，等待集成选择。

**用户最新范围（2026-10-10，优先于下文旧验收要求）：只使用并验证 PostgreSQL。不运行 SQLite/MySQL 检查、矩阵、迁移测试或其 race；旧“三库/五引擎/最低 MySQL 版本”验收已撤销。数据库验证只用任务专用 PostgreSQL，必须真实运行且不得跳过；纯单元测试、类型检查、构建、安全与业务验证仍保留。不要运行会隐式启动 SQLite/MySQL 的宽泛测试命令，也不要为了全量测试把无关历史测试重写为 PG。保留现有生产跨库代码，不为本任务扩展或删除它；不得宣称跨库验证完成。CI 新增门禁仅 PostgreSQL。继续业务实施，不因其他数据库未检查而阻塞。**

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 各环境超级管理员在 Web 拉取 dev 的供应商、模型元数据和定价，预览字段级 changelog，确认后安全受管同步并可恢复。

**Architecture:** 固定 dev 只读来源，由目标后端拉取完整快照并保存有期限的计划。纯契约与三方差异算法放在无数据库依赖的包，model 层持有事务、基线、版本、备份及运行时发布屏障，service/controller 负责网络和会话授权。复用 CLI 的记录映射，不复用其 PostgreSQL 限定 apply 或整块覆盖语义。

**Tech Stack:** Go 1.26.5（当前 go.mod 与实际测试工具链）、Gin、GORM、PostgreSQL（本任务唯一数据库验收范围；既有其他数据库生产代码不扩展或删除）、React 19、TypeScript、TanStack Query/Table、现有安全验证与 Base UI 组件、Bun/Vitest。

**Spec:** `.ccg/tasks/web-dev-catalog-sync/design.md`（用户已批准）。

## Global Constraints

- 只从 https://dev.molii.co 拉取；15 秒超时；解压后响应上限 10 MiB；服务端计划有效期 10 分钟。
- A 受管同步：目标独有数据保留；仅可信完整来源移除的已受管项可以提出删除；删除需单独确认，危险回退或仍被引用时阻断。
- 同步供应商、持久化模型资料、计费币种、原始表达式、插件覆盖和明确的专项价格；不改渠道、用户、余额、密钥、品牌、汇率、充值或分组。
- 本地修改冲突必须明确覆盖；首版整批应用，不提供逐字段部分应用。
- 首版只支持每站一个应用实例；检测多实例/未知部署形态时禁止应用。部署不等于授权首次同步生产数据。
- 业务 JSON 使用 common 包装；不新增生产依赖；不建立第二套计费规则；不自动转换传统定价或凭空补造元数据。
- 本任务只验证真实 PostgreSQL，必需的 PostgreSQL 集成测试不得跳过；不检查 SQLite/MySQL，不新增其 CI 矩阵，不宣称本次完成跨库验收。
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
- `deploy/env/{development,production-molii,production-ixiaozu,production-claudeye,production-model-claudeye}.env.example`：非秘密配置说明；`deploy/tests/deploy_test.sh` 验证 runtime 文件保留；`.github/workflows/deploy.yml` 只增加 PostgreSQL 功能验收门禁，不增加其他数据库检查。

## Shared Interfaces

`pkg/catalogmanifest` 定义以下公开类型，后续任务不能自行改名：

- `Actor{UserID int; SessionID string; TargetID string; AuthVersion int; SessionVersion int}`，由现有会话身份显式映射，不保存会话 token。
- `Entry{Kind string; Key string; Value string}`：Kind 为 vendor/model/model_price/plugin_price/special_price/tool_price；Value 为规范 JSON，Key 不含数据库 ID；实际叶子路径用 JSON Pointer 而非点号拼接，防止含点模型名冲突。
- `Snapshot{SchemaVersion int; SourceID string; ExportedAt int64; Digest string; Complete bool; Capabilities map[string]string; Entries []Entry; Coverage map[string]int; ObjectVersions map[string]string}`：版本 2；价格类别 Key 必须结构化编码，区分 plugin key 与模型名；覆盖计数参与完整性验证；可选 ObjectVersions 是本地不透明代次令牌，不输出数字 ID。
- `Baseline{SourceID string; Generation int64; Entries []Entry; ObjectVersions map[string]string}`：Value 是上次成功应用值；ObjectVersions 识别目标记录代次，不把软删除后新记录自动当成原对象。
- `Resolution{OverwriteKeys []string; ConfirmDeletes bool}`；`Change{Kind,Key,Action,Reason string; Before,Base,After *Entry}`；Action 为 create/update/delete/adopt/conflict/preserve/blocked/unchanged。元数据 Change 值只含可变业务字段；完整快照保留来源审计时间。
- `Plan{ID string; Snapshot Snapshot; TargetDigest string; BaselineGeneration int64; Actor Actor; ExpiresAt int64; Resolution Resolution; Digest string; Changes []Change}`。
- `Result{OperationID,State string; Revision int64}`：State 为 committed_pending_publish/succeeded/failed；失败错误不泄露凭证。

所有摘要均基于规范化内容；快照内容摘要忽略 ExportedAt，最终计划摘要绑定完整固定快照（含时间版本）、操作者版本、计划 ID/有效期、目标、覆盖范围、兼容版本、所需引用与明确确认的选择。外部请求不直接反序列化成可执行 Plan。实际类型以已审查的 pkg/catalogmanifest/types.go 为准。

### Task 1: 固化 v2 快照契约和完整导出

**Files:** 新建 `pkg/catalogmanifest/types.go`；v2 导出可独立放在 `internal/catalogsync/managed_snapshot.go`，保留 `snapshot.go` 的 v1 CLI 行为；扩展现有 `internal/catalogsync/catalogsync_test.go`。

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

**Files:** 新建 `model/catalog_sync.go`、`model/catalog_sync_store.go`、`model/catalog_sync_snapshot.go`、`model/catalog_sync_test.go`；修改 `model/main.go`、现有元数据/价格写入路径（见 ownership map）。将现有 v2 导出投影移到 model 所属快照文件，`internal/catalogsync/managed_snapshot.go` 仅保留委托桥，避免 model→internal 循环依赖；不复制第二套投影或白名单。

**Interfaces:** `model.CreateCatalogSyncPlan(ctx context.Context, source catalogmanifest.Snapshot, actor catalogmanifest.Actor, now time.Time) (catalogmanifest.Plan,error)`；`model.GetCatalogSyncPlan(ctx context.Context, id string, actor catalogmanifest.Actor) (catalogmanifest.Plan,error)`；`model.WithCatalogReadSnapshot(ctx context.Context, read func(*gorm.DB) error) error`。

- [ ] 写 `TestCatalogSyncStore` 和 `TestCatalogSyncSourceConcurrentMutation`：10 分钟期限、会话/目标绑定、全同值接管、完整来源快照、防软删除后身份混淆、升级后两次迁移无额外 schema 变化。
- [ ] 三库运行 `go test ./model -run 'TestCatalogSync(Store|SourceConcurrentMutation)' -count=1 -v` 确认红灯；测试使用既有 `TEST_MYSQL_DSN`/`TEST_POSTGRES_DSN` 和真实 SQLite。测试 DSN 必须指向临时测试库。
- [ ] 建立 `CatalogSyncState`（固定主键版本/发布状态）、`CatalogSyncPlan`（固定快照、选择、会话绑定、期限）、`CatalogSyncBaseline`（受管条目、对象代次）、`CatalogSyncOperation`（备份、历史、幂等结果）表。大正文用兼容 TEXT/长文本策略；复合身份索引用规范 Key 的固定 64 字符摘要，避免 MySQL 旧版本索引过长。
- [ ] 所有目录/价格写入协调同一 model 层锁与 revision；SQLite 使用明确串行写入、MySQL/PostgreSQL 使用共享 model 锁方法。固定状态行先初始化再锁，不能锁不存在行。标准保存、排序、官方资料同步、渠道导致的目录自动补齐都必须纳入；源导出在此屏障和只读事务内获得一致快照。
- [ ] 同命令三库转绿，再回归原模型/供应商写入测试；提交 `feat: persist managed sync plans and ownership baselines`。

### Task 4: 有保护的事务应用与最近一次恢复

**Files:** 新建 `model/catalog_sync_apply.go`，引用围栏/验证/恢复可按职责拆入 `model/catalog_sync_references.go`、`model/catalog_sync_validate.go`、`model/catalog_sync_restore.go`；扩展 `model/catalog_sync_test.go` 及 Task 3 存储/快照文件。必要修改 `model/model_pricing_config.go`、`model/task_model_alias.go`、`model/task_plugin.go` 以复用事务内完整草稿与别名/schema 读取；`pkg/jsplugin/registry.go` 及对应测试提供贯穿提交的代次读保护；`pkg/jsplugin/engine.go` 及对应测试保留实际编译源字节的不可变 SHA-256 身份，以校验持久化所需插件与实际生效程序一致。修复已复现的必要 Channel MySQL TEXT-default 迁移兼容问题。不调用会提前发布运行时状态的普通保存函数，不同步插件程序。

**Interfaces:** `model.ApplyCatalogSyncPlan(ctx context.Context, planID, finalDigest, operationID string, actor catalogmanifest.Actor) (catalogmanifest.Result,error)`；`model.CreateCatalogSyncRestorePlan(ctx context.Context, operationID string, actor catalogmanifest.Actor, now time.Time) (catalogmanifest.Plan,error)`；`model.ListCatalogSyncOperations(ctx context.Context, offset,limit int) ([]CatalogSyncOperation,int64,error)`。

- [ ] 写 `TestCatalogSyncApplyAndRestore`：备份写入失败全回滚；普通改价竞争返回冲突；确认后 dev 更新不影响固定快照；同操作重复提交无重复；价格与币种原样；仍被渠道/映射/未完成任务引用阻止删除；供应商被独有模型引用阻止删除；恢复不覆盖后续本地修改；同名重建不误删。
- [ ] 三库运行 `go test ./model -run 'TestCatalogSync(ApplyAndRestore|Store)' -count=1 -v` 确认红灯。
- [ ] 在统一锁内重验计划/对象代次/revision/全部引用，验证目标现有插件绑定和完整价格草稿；按供应商→模型→价格→安全删除→基线顺序写入，并同事务保存前值备份和操作 ID。无法解释的渠道匹配、专项回退或插件 schema 直接 blocked，不猜测兼容性。
- [ ] 对受影响的未完成旧任务，若无法证明计费价格已冻结或此次变化不影响其结算，阻断相关危险改价/删价；不通过修改任务或臆造历史快照补救。与在途冻结和“不修改任务”两个约束一致；无关目录变化不因此一律阻断。
- [ ] 持久化引用并发保护可使用数据库事务围栏而非侵入所有渠道/任务写入者；必须以真实三库证明覆盖插入空隙与绕过应用锁的写入。PG 使用明确 READ COMMITTED 和可信关系表锁；MySQL 使用已验证 InnoDB/REPEATABLE READ 的无筛选完整索引锁定扫描，不能在普通一致性读取之后才加围栏并继续用旧视图；SQLite 先取得写事务。未知引擎/配置、缺表/不完整扫描、嵌套事务、超时/死锁均回滚并阻断，禁止使用会隐式提交的 MySQL LOCK TABLES。插件持久化与实际代次同时校验，代次读保护贯穿真正 COMMIT，提交后先释放再发布选项，避免反序锁。
- [ ] 按设计只生成最近一次成功操作的逆向计划，复用相同确认和应用流程；恢复后还原此次受管基线但不回退全站 revision。新建对象的恢复删除也执行引用保护。备份和历史不含完整站点 options。
- [ ] 三库转绿；提交 `feat: apply and restore managed catalog transactions safely`。

Task 4 实施细分（串行；每段独立提交及审查，全部完成才算 Task 4 完成）：

- 4a：必要 Channel 跨库迁移、实际编译源 SHA-256 身份、贯穿提交的 fail-fast registry lease；不增加可执行 apply 入口。迁移必须 fresh/released upgrade/两次无 DDL 重复；lease 必须证明排斥代次切换、释放、缺失/不同程序身份和 retained/pending 状态。
- 4b：独立根事务引用围栏；明确方言/隔离和 first-read 顺序，真实三库直接连接写入的空表/非空表 insert/update/delete、状态切换、旧快照陷阱、超时及失败关闭测试。复用 4a lease；不提前接入未经验证的业务写入。
- 4c：事务内引用投影、完整合并草稿校验、备份/应用/幂等/恢复及权威操作者验证；基于已审查 4a/4b 完成 Task 4 全部测试，提交后状态仅 committed_pending_publish，直到 Task 5 真正发布。
- 4c 的完整草稿校验复用现有纯 validator，但其编译/烟雾求值在围栏事务外暂存；事务内以权威完整草稿、别名/schema/插件实际身份输入的逐项相等重检确认同一校验结论，变化即拒绝，不允许校验 A 后写 B。暂存结论及依赖摘要由服务端封存，不运行插件/表达式编译或钩子于4b围栏内。
- 4c 继续串行细分并独立审查：4c1 显式 validator 依赖/结构校验路径/服务端 attestation 与摘要 DTO；4c2 引用投影/实际预览封存/事务应用/备份/鉴权/幂等；4c3 封存逆向恢复与安全历史投影。全部完成才算4c/Task4完成，不提前暴露 API。
- 4c1 允许对 `pkg/catalogmanifest/{types.go,diff.go,diff_test.go}` 增加服务端 Plan.Kind(sync|restore)、RestoreOperationID、ValidationDigest、ReferenceDigest 及摘要绑定/结构校验复用；`CatalogSyncPlan.Validation` 为私有持久化 payload。公开完整验证语义不弱化，结构-only 不能单独授权应用。实际类型、迁移及测试以审查后代码为准。
- 4c3 安全历史保留既定 List 返回签名，通过 `gorm:"-"` 摘要字段和清空私有 storage 字段投影；不泄露 sessionID/备份/源码。撤销首次同步后允许 SourceID/Entries/ObjectVersions 皆空而 Generation 安全递增的空所有权基线；只窄幅修改 `pkg/catalogmanifest/{diff.go,diff_test.go}`，仍拒绝负代次、无来源的非空数据/代次令牌及来源变更，纯边界和实际再次预览回归必需。

### Task 5: 运行时批量发布、计费快照与崩溃恢复

**Files:** 新建 `model/catalog_sync_runtime.go`；修改 `model/model_pricing_config.go`、`model/option.go`、`model/model_billing_money.go`、`model/pricing.go`、`model/pricing_refresh.go`、`relay/helper/price.go`、`relay/helper/billing_expr_request.go`、`service/task_billing.go`、`main.go`；扩展 `model/catalog_sync_test.go`，沿用已有 relay/task 测试文件补关键快照回归。为覆盖实际价格选择边界，另允许窄幅修改 `relay/relay_task.go`、`relay/common/{relay_info.go,tool_usage.go}`、`service/{text_quota.go,grok_video_billing.go,quota.go}`、`controller/relay.go` 的图像选择编排、`relay/channel/moliigrok/adaptor.go`、`relay/channel/task/{moliigrok,starai}/adaptor.go`、`setting/operation_setting/{tools.go,molii_grok_tool_price.go}`、`setting/ratio_setting/{molii_grok_price.go,starai_video_price.go}` 及对应已有测试；如不可变选择放在 PriceData，允许 `types/price_data.go`。不新增第二套计费求值器、不改变定价业务规则、不凭空接入没有生产调用者的 task factor。

**Interfaces:** `model.PublishCatalogSyncRevision(ctx context.Context, revision int64) error`；`model.RecoverCatalogSyncRuntime(ctx context.Context) error`；`model.WithCatalogPricingRead(ctx context.Context, modelName string, capture func() error) error`。Task 4 在提交后调用发布，只有发布成功才将操作标记 succeeded。

- [ ] 写 `TestCatalogSyncAtomicCurrencyPrice`、`TestCatalogSyncCommittedBeforePublishRestart`、`TestCatalogSyncResponseLostRetry`：不能观察到新币种/旧表达式；删除后内存无旧条目；提交后崩溃启动恢复；重复操作查询原结果；发布失败只阻断受影响新请求；在途任务继续旧快照。
- [ ] 运行 `go test ./model ./relay/helper ./service -run 'TestCatalogSync|Test.*BillingSnapshot' -count=1`，确认红灯。
- [ ] 发布前构建并校验整批运行时配置；常规保存和后台 SyncOptions 加入同一屏障。读屏障仅包住解析及冻结价格快照，不跨网络、流或任务生命周期持锁。同步发布重建删除项默认状态并失效公开定价/元数据缓存。
- [ ] 运行时候选重建与 Web 目录导出是不同验证边界：复用原始价格解析/现有保存策略，不把 Web“每项价格必须有可靠元数据币种”的导出限制扩散到所有历史普通保存/启动而导致整站无法初始化。Web 同步仍严格阻断无可靠币种的价格；不得通过默认 USD 或补造元数据绕过。保留旧行为不等于将其宣称为可同步数据。
- [ ] 同一个短读屏障同时捕获基础表达式/传统价格、币种、专项单价与其同代次锚点、完整有效工具价格查找状态。图像估价先于 helper、任务估价晚于 helper 的现有顺序不能各自独立加锁后声称原子；插件钩子/媒体处理在屏障外使用捕获值。工具计数与最终附加费读取同一请求快照，显式空/零不回退实时配置；音频/realtime 已有 PriceData 比率的消费者不再重读同步配置。历史不完整任务沿用明确边界，不伪造旧价格。
- [ ] committed_pending_publish 持久化并在启动初始化选项后、接收请求前恢复；发布失败不得报成功。使用当前 SystemInstance 心跳和明确单实例配置双重检查，无法验证单实例则关闭 apply；不声称已支持多节点一致性。
- [ ] 相关测试转绿并运行 `go test -race ./model ./relay/helper ./service`；提交 `feat: publish synchronized pricing snapshots without restarts`。

Task 5 实施细分（串行独立审查；全部完成才算 Task 5 完成）：

- 5a：构建不修改实时状态的完整候选、校验、默认值重建及批量发布，持久化发布状态/恢复与单实例门禁，复用常规保存/reload 路径。`runtime-publication-research.md` 已确认原通用 loader 会静默跳过错误且不重置缺失项，不能用它的 nil 返回声称成功。允许对已有 `setting/billing_setting/tiered_billing.go`、`setting/ratio_setting/{model_ratio.go,cache_ratio.go,molii_grok_price.go,starai_video_price.go}`、`setting/operation_setting/{tools.go,molii_grok_tool_price.go}`、`setting/task_pricing_setting/config.go` 增加包所属的默认值副本/已验证数据发布支持；`types/rw_map.go` 只为必要的 typed replacement 修改。不得在 model 复制默认表或重注册全局指针假装发布。必要启动插件就绪编排只在原调用位置调整，不下载插件。另须修改 `model/main.go` 必要启动顺序：migrateDB 的目录 backfill/排序/币种数据迁移在 pending 时会先于恢复被普通写入门禁拒绝，需安全延后到真实恢复之后；重启回归必须覆盖实际初始化链，不能仅直接调用 Recover 函数。
- 5a 再串行细分独立审查：5a1 包所属不可变默认值、完整 detached typed candidate 与 no-fail publication primitives（新 `model/catalog_sync_candidate.go`/对应 focused test）；5a2 持久化发布生命周期与常规保存/reload 集成；5a3 实际启动恢复/插件就绪和权威心跳单实例门禁。5a1 的纯测试不替代后两段跨库与实际初始化证明；prospective 新值不可当成自身 previous 来启用 stale-override 保留豁免，复用既有 validator 且显式区分 committed reconstruction。全部完成才算5a；不提前开放接口。
- 5b：基于 5a 发布屏障，把真实基础/专项/工具/音频请求价格选择收敛为同代次不可变快照，并让估价、计数和结算复用。具体遗漏与文件见 `special-price-capture-research.md`；不得把锁扩大到钩子、网络或任务生命期。全链路跨代次和旧在途请求回归完成后才开放 Task 6 应用 API。
- 5b 再串行独立审查：5b1 共享多身份读门禁、不可变请求选价与实际基础/文本/工具/音频；5b2 图像估价与重试编排；5b3 任务提交、Seedance/Grok 估价及持久化快照。全部三段及合并回归完成才算5b，不提前开放接口。5b1 窄幅允许 `model/catalog_sync_runtime.go` 复用原读门禁校验完整身份集，新增 `relay/common/pricing_selection.go` 与 `relay/helper/catalog_pricing.go` 分离不可变输入和短选价；不复制计费引擎。新消费测试放在无旧数据库 TestMain 的 relay 包；必要 service TestMain 只增加显式 PostgreSQL 入口，禁止运行 SQLite/MySQL 或重写无关历史用例。
- 5a2 再串行分为5a2A已提交状态捕获/重检、可报错派生重建与发布/恢复/读门禁，5a2B prospective apply/restore及所有普通写入/reload集成。5a2A不提前接通普通/同步写入或声称整体完成。5a2B额外允许窄幅修改 `model/passkey_option.go` 和新增对应focused test，覆盖现有passkey/catalog混合事务；只改发布协调，不改变域名预览/确认/HMAC/RP/origin鉴权语义。
- 5a2B串行验收B0只读渠道解析/组索引锁前置保护（窄幅 `model/channel.go`/`model/pricing.go`/runtime focused test），B1严格prospective apply/restore，B2全部普通/mixed/reload集成。额外窄幅允许 `model/ability.go`/`model/channel.go`复用上下文查询，不改普通查询/渠道业务规则。未完成全部子段不得开放同步入口或称整体发布已完成。
- 5a2B2再串行验收B2a通用/专项/批量及元数据共享事务发布，B2b模型定价保存的独立准备/版本重检，B2c混合passkey原子保存和权威reload；额外窄幅允许 `model/model_metadata_sync.go`、`model/marketplace_display_order.go` 的实际共享wrapper协调，不改个别vendor/reconcile业务。每段独立审查，全部通过才算B2；启动共享wrapper的schema/plugin就绪与pending顺序仍由5a3实际初始化验证，不加绕过。
- B2追加必需串行B2d：普通“删除模型并从渠道移除”会改变任务别名依赖，须基于实际删除前像，在锁外复用现有别名聚合的纯投影并在最终事务内精确重检、只执行一次原删除回调。窄幅允许 `model/model_meta.go`、`model/task_model_alias.go` 及已列runtime/store/shared-wrapper路径与focused测试；不复制别名引擎，不放宽任意依赖漂移、不在锁内编译，不将普通用户明确的渠道移除扩展为目录同步的渠道写入权限。B2a新增真实失败用例先保存在本任务scratch，B2d恢复并重现RED后实施；四段均通过独立审查才算B2，不能忽略原有后台功能回归。

### Task 6: 来源只读认证、网络限制和超级管理员 API

**Files:** 新建 `service/catalog_sync.go`、`controller/catalog_sync.go`、`controller/catalog_sync_test.go`；修改 `service/security_verification.go`、`router/api-router.go`；扩展既有 `controller/security_enrollment_test.go` 验证 scope；配置在新 service 以受控环境读取，不放普通 options。必要窄幅修改 `model/catalog_sync_snapshot.go` 及已有 sync tests，复用已审查物理表检测以拒绝来源导出的非事务表、RLS/视图/遮蔽等不完整安全视图；导出一致性事务内保持实际关系/引擎验证稳定，不修改源数据。

**Interfaces:** `service.FetchDevCatalog(ctx context.Context) (catalogmanifest.Snapshot,error)`；管理根路径 `/api/catalog_sync`：`GET /status`、`POST /preview`、`POST /plans/:id/resolve`、`POST /plans/:id/apply`、`GET /history`、`GET /operations/:id`、`POST /operations/:id/restore-preview`。只读来源为 `GET /api/catalog_sync/export`，独立专用凭证认证，不走管理登录认证。

- [ ] 实施前完整阅读项目要求的 OWASP Authentication、Session Management、CSRF 指南及相关当前 ASVS 控制，将引用和未满足项记入本任务 review.md，不声称未经验证的合规。
- [ ] 写 `TestCatalogSyncHTTP`：普通管理员/个人 token 不可应用、错误/撤销来源凭证拒绝、token 不出日志/响应、私网/DNS 重绑定/重定向拒绝、15 秒超时和解压后 10 MiB 限制、CSRF、会话过期、10 分钟计划过期、安全证明错误 scope/context/过期/重放、只读导出不能写其他接口。
- [ ] 运行 `go test ./controller -run 'TestCatalogSyncHTTP|Test.*Security' -count=1`，确认红灯。
- [ ] 固定 HTTPS dev 地址及受限 DialContext，校验解压后大小；源以独立凭证哈希列表认证并限流，不降级公开 pricing。注册 `catalog.sync.apply` 与 `catalog.sync.restore` scope，绑定计划最终摘要和目标；沿用 RequireSecurityProof，不复制渠道读取 scope。
- [ ] 来源的 complete 声明必须基于可信物理完整视图：来源 models/vendors/options 的非事务引擎、RLS 过滤、视图、schema/临时表遮蔽或验证后关系切换不得被普通 repeatable-read 包装误当成完整 MVCC 快照。复用必要物理关系检查，事务内稳定验证，真实源库负例证明拒绝；不增加整站锁或秘密导出。
- [ ] 环境契约：`CATALOG_SYNC_ROLE=source|target|disabled`（默认 disabled）、`CATALOG_SYNC_SOURCE_ID`（稳定 dev 身份）、目标 `CATALOG_SYNC_TARGET_ID`、目标秘密 `CATALOG_SYNC_TOKEN`、源 `CATALOG_SYNC_READERS_JSON`（凭证 ID→哈希及状态）、`CATALOG_SYNC_SINGLE_INSTANCE=true`。只读 status 返回角色/就绪状态/非秘密标识；v1 凭证通过受保护 runtime 环境配置及轮换，不新增 Web 明文密钥保存流程。
- [ ] resolve 保存冲突覆盖/删除确认并生成最终摘要，apply 请求只接收 plan ID、摘要、operation ID 和安全证明，不接收 Snapshot。HTTP 409 用于版本冲突，422 用于阻断，503 用于来源/发布不可用；超时客户端查询 operation ID，不自动重放安全证明。
- [ ] 安全/API 测试转绿；提交 `feat: expose authenticated managed catalog synchronization APIs`。

Task6 按实际公共接口缺口串行细分并独立审查：6a 可信来源只读快照、当前超管会话认证的精确回执查询与安全历史单项查询；6b 独立凭证/配置/固定 dev 网络边界；6c scope/会话/严格来源策略；6d HTTP 编排、失败审计及最终路由注册。追加窄幅文件归属 `model/catalog_sync_apply.go`、`model/catalog_sync_restore.go`、`model/catalog_sync_references.go`，仅复用原私有验证器/历史投影及物理关系和 namespace 规则，不调用会写 anchor 的目标事务来假装只读。6a 的新源导出自持 SQL READ ONLY REPEATABLE READ；回执查询复用 FOR UPDATE 鉴权，普通 READ COMMITTED、无目录写入，允许精确已提交回放独立于计划过期/删除/发布状态。追加非秘密 `CATALOG_SYNC_EXTERNAL_ORIGIN`，精确 HTTPS 外部来源、缺失/格式错误失败关闭；Origin 缺失按既有严格 Referer 来源解析，不信任任意 Host/Forwarded。整个6完成前不开放管理路由。

Task6 测试命令以 PostgreSQL-only 范围和安全 fixture 为准，旧 `Test.*Security` 宽泛 selector 不得直接执行。`model` 及其他已知不安全 TestMain 入口不得因 `/postgres$` 过滤而绕过限制；新控制器用例必须使用任务专用真实 PG，逐个审查选中 fixture 的初始化，不重写无关历史测试。各子段实际证据以 report/独立 review/账本为准，预案报告不算实施或通过记录。

6a 首次独立审查发现并由真实 PostgreSQL 复现 ACCESS SHARE 下并发继承以及 schema 换名/换回 ABA 风险。新只读 helper 允许保守乐观元数据版本校验：原物理关系/锁证明之外记录 pg_class/pg_namespace 的 OID/xmin/ctid，拒绝 relhassubclass 历史 true hint，在读提交后同一保留连接的全新快照重检，通过后才返回任何可信结果；失败返回零结果。源 SQL READ ONLY/SELECT-only 权限和普通 DML 并发要求不变，维护造成误拒绝的限制须披露。目标写入路径也共用 namespace 名称规则，不能用提交后检查补救；此风险必须另有最小提交前证明提案、实施和独立审查的前置 gate，不得仅修读接口便开放管理路由。追加目标路径归属尚未批准，以具体提案为准，不扩大到其他数据库。

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

- [ ] 只使用临时真实 PostgreSQL 和最小权限测试 DSN，运行完整功能的 PostgreSQL 用例，必需用例不得 SKIP。fresh/旧版代表性 PostgreSQL 库升级均跑两次迁移并检查原数据/索引；不得复用生产库。CI 预先验证缺 PostgreSQL DSN 会失败。
- [ ] 针对完整引用围栏的 O(历史行数) 扫描，用临时 PostgreSQL 记录较大历史规模下的行数、耗时及超时后完整回滚/释放证据；实际量级和未知生产规模明确披露，不能以缩窄扫描或取消围栏改善数据。不运行其他数据库或最低版本矩阵。
- [ ] 运行经筛选不会隐式执行其他库 fixture 的纯单元测试、完整功能 PostgreSQL 回归与受影响 PostgreSQL race，以及 `go build ./...`；web 完整 test/typecheck/受影响 lint/i18n/build；部署脚本 `bash deploy/tests/deploy_test.sh`，确认原 runtime secrets 不被部署覆盖。不为了全量测试重写无关历史 fixture。
- [ ] 审查权限绕过、SSRF、来源完整性、所有权删除、并发写入、缓存发布与恢复，记录真实命令、PostgreSQL 版本、失败/修复和未解决问题到 review.md；不把未检查其他数据库视为阻塞，也不宣称跨库通过。
- [ ] 部署配置默认 disabled，dev 配只读源、各目标单独配置凭证，预览导出不得带秘密。设计代码提交并不授权 push、生产发布或首次目录 apply；获得相应用户指令后才执行。
- [ ] 发布时先 dev 再目标；仅验证连接、只读预览、权限和健康，不付费调用上游，不自动 apply。目标具体域名在部署前确认，无需为写功能猜测三站名单。
- [ ] 最后核对精确 git diff、无秘密、无用户 ` 2` 文件，归档本 CCG 任务并提交。只有实现、审查、必要验证与获授权的交付均完成才标记 completed。

## 执行依赖和评审结果

顺序：1 → 2 → 3 → 4 → 5 → 6 → 7 → 8。网络/UI 可以在契约冻结后用 native Codex 子代理按文件分工准备，但主干 model 事务/发布文件必须单人串行，controller 与 service 的相互依赖不可各自猜测接口。不得省略最终整条链路验证。

自审：设计 1–3 由 Task 1/6/7 覆盖；设计 4 由 Task 2/4 覆盖；设计 5–6 由 Task 6/7 覆盖；设计 7 由 Task 3/4 覆盖；设计 8 由 Task 5/4 覆盖；设计 9 的数据库和发布边界由 Task 8 覆盖。五项 Review Focus 均已指定回归测试；shared interfaces 使用同一包，避免 model→internal 导入环。尚未运行上述产品测试：它们是实施验收命令，不是通过记录。

用户已批准在现有隔离工作区使用原生 Codex 子代理实施。任务1–4、整个5a（含5a3实际启动/插件/实例）、整个5b（共享请求、文本/工具/音频、图像、任务持久快照）、整个6（6a只读来源/回执/历史及修复、6aT目标提交前校验、6b凭证/网络、6c安全策略、6d实际HTTP及修复）均已本地提交并通过独立审查，HEADbdc42af8f；当前实施7超级管理员环境同步页面，随后8 PostgreSQL及构建/前端完整验收。只验证 PostgreSQL，不检查 SQLite/MySQL。已发现并禁止 model/task_cas_test.go 隐式初始化SQLite的测试入口，早先相关“仅PG”证据说法已在review/账本纠正，最终验收须先采用明确PG隔离入口。上方原始任务清单保留需求和历史命令，冲突要求由开头用户范围覆盖；逐子段实际完成状态以本任务 task.json、review.md 和实施账本为准，不用原计划命令代替实际测试证据。未调用外部执行器，未推送、部署或进行线上目录同步。
