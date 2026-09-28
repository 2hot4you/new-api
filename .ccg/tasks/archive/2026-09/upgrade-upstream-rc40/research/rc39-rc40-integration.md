# rc.39 → rc.40 集成研究

## 核对基线与结论

- 研究时 `HEAD == origin/develop == 3f8d06115d757761442166142984ea47fd393f04`，仍以 rc.36 为最近官方发布祖先；本报告以“rc.37/rc.38 兼容检查点完成后”为实施前提，并非声称已完成这些合并。
- 官方标签：rc.38 `2906e4f779`，rc.39 `9978ee1e25`，rc.40 `0aec08fee8`。rc.38→39 为 243 文件、15232 行新增/6738 行删除；rc.39→40 为 52 文件、1992 行新增/247 行删除。统计直接来自本地官方 tag 的 Git diff。
- 已读根 `AGENTS.md`、前端规范与任务 requirements；本 worktree 没有 `.ccg/spec/`。只写本研究文件，没有改业务代码、运行合并或部署。
- 最危险的交集是：图片协议改由插件处理、异步资金结算、重复性能采样、任务列表省略 Data、预扣费计算默认变化、MySQL LongText 迁移。rc.40 不是单纯 UI 修复。

## Files Found：准确文件组与实施归属

| 组 | 必须一起考虑的文件 | 上游变化 / Molii 交集 |
| --- | --- | --- |
| A：模型归属与渠道身份 | `service/channel_select.go`、`middleware/distributor.go`、`middleware/task_plugin.go`、`model/channel_constraint.go`、`relay/relay_task.go`、`constant/channel.go` | 保留 61 StarAI、62 Molii Grok、63 Task Plugin、64 ByteDance Seedance；官方 39 只修改 Doubao 名称，不授权重新编号。37/38 分配的新渠道 ID 必须沿用前一检查点结果。 |
| B：插件元数据与绑定 | `pkg/jsplugin/{registry,routing}.go`、`relaykit/dto/channel_settings.go`、`model/task_plugin.go`、`controller/{channel,task_plugin}.go`、`relay/channel/task/jsplugin/adaptor.go` | 增加 `meta.upstreams`、`UsageForModels`、`task_extend_plugin_keys`、结果保留标志、多文件引用；New API 渠道 60 可绑定多插件。 |
| C：OpenAI Images bridge | `router/{relay-router,task-plugin-protocol-router}.go`、`controller/{relay,plugin_protocol,plugin_protocol_image}.go`、`relay/image_handler.go` | `/v1/images/generations`、`/v1/images/edits` 改由 host protocol 路由注册；被插件认领则运行 task bridge，否则 fallback 到原 Image relay。`/v1/edits` 旧路径仍保留。 |
| D：供应商实现 | `plugins/tasks/{alibaba,doubao,jimeng,kling,sora,sunoapi}/plugin.js`、`plugins/{alibaba_responses,doubao_responses,builtin_plugins}_test.go`、`relay/channel/ali/*`、`setting/model_setting/qwen.go` | Ali 旧 image Go 实现和 Qwen 同步配置删除；Alibaba 插件实现图片协议。Doubao 加 Seedream、Seedance capabilities/profile，但仍需保留 Molii native adaptors。 |
| E：图片和通用预扣费 | `setting/operation_setting/quota_setting.go`、`common/{quota,constants,model}.go`、`model/option.go`、`relay/helper/{price,billing_expr_request,valid_request}.go`、`service/{billing_session,image_billing}.go`、`pkg/billingexpr/{types.go,expr.md}` | 上游输入费用预扣倍率/信任阈值、移除 Ali prompt_extend 特殊逻辑；与 Molii 映射后计费身份、Grok 图片快照、GPT Image 2 价格和预留相交。修改表达式链前必须读完整 `pkg/billingexpr/expr.md`。 |
| F：持久结果与 DTO | `model/task.go`、`dto/task.go`、`controller/task.go`、`service/task_billing.go`、`relay/relay_task.go`、`middleware/task_plugin.go`、`controller/plugin_protocol.go` | `ResultDiscarded` / `ResultRetrievable`，Insert 可 omit data，查询结果不可再取，列表不带 data；Molii Timing/InputMedia/VideoStudioRequest/StoredResult/BillingContext 必须全部保留。 |
| G：终态/outbox/perf | `service/{task_polling,task_billing,task_billing_reconcile}.go`、`model/{task_billing_job,task_billing_tx}.go`、`pkg/perf_metrics/metrics.go` | 官方新增终态采样与 direct settle；Molii 已有终态 CAS 与 outbox 原子入队、fenced reconcile 和结算后采样，必须语义合并。 |
| H：rc.40 插件存储/同步 | `model/task_plugin.go`、`controller/task_plugin.go`、相关已有测试、`model/{migration_dialector,migration_dialector_test,postgres_migration_integration_test}.go` | Source/Icon 变 LongText，上传 1→8 MiB；sync snapshot 不再装载 source/icon，只为 changed rows 按 ID 读源码。 |
| I：relaykit 转换 | `relaykit/relayconvert/internal/{claude_messages/to_oai_chat_req,oai_responses/to_oai_chat_req,oai_chat/to_oai_responses_resp,oai_chat/to_oai_responses_stream_resp}.go` 与对应 tests，`relaykit/dto/claude.go` | rc.39 tool-name 查询优化；rc.40 tool-output 媒体移至 user message、tool batch 连续、reasoning/text 多段 SSE 生命周期修复。 |
| J：渠道/插件前端 | `web/src/features/channels/{components,lib,hooks}`、`features/task-plugins/{components,lib,__tests__}`、`features/system-settings/general/quota-settings-section.tsx`、对应 types/locales | 多插件配置、绑定权限、上游模型更新预览、上传激活入口、metadata 安全常量解析和 8 MiB cap。保留 61/62/64 的管理字段、模型选择和密钥配置。 |
| K：价格/日志前端 | `features/pricing/{components,lib,__tests__}`、`features/system-settings/{billing,models}`、`features/usage-logs/{components,lib,types.ts}` | 任务 schema/profile 和 enum 缩窄后的可编辑价格，result_discarded/task_sync/image_count，response-model 展示。与 Molii CNY/USD、Grok/Seedance 专用价格表、marketplace 卡片、实际用量详情有大量重叠。 |
| L：主题/品牌/部署 | `web/src/context/theme-*.tsx`、`web/src/lib/{theme-storage,theme-customization,frontend-cache}.ts`、`web/{index.html,rsbuild.config.ts,public/favicon.ico}`、`web/src/config/site-brand.ts`、`web/build/site-brand.ts`、`web/src/styles/*`、`deploy/*`、`.github/workflows/ci.yml` | 官方 logo/favicon 改版、偏好 cookie→localStorage；Molii 使用运行时品牌和锁定默认主题、多站点部署。不可整文件覆盖。 |

## Dependencies：顺序与调用链

1. rc.38 检查点必须先固定渠道 ID、适配器注册和 relaykit API；记录 checkpoint commit。新行为测试在此基线先 RED，记录预期失败原因。
2. 合并官方 rc.39（保留其 ancestry），先接 B：metadata/protocol/DTO/settings，再接 A 的过滤与执行身份，然后接 D 供应商驱动。J 可在 B 契约固定后并行；I 可独立。
3. C 依赖 B/A/D；F 依赖 B，且要先解决列表 Data 的 Molii consumers；E 与 G 共享账务状态，不能让两个写代理同时修改 `relay/relay_task.go`、`controller/relay.go`、`service/task_billing.go`、`service/task_polling.go`。
4. 完成 C/F/E/G 后接 K 与 shared components/locales，运行后端/前端/数据库/部署契约门禁，产生 rc.39 兼容 checkpoint。
5. 此后再合并 rc.40：H（model 类型→controller casts→snapshot/source loading），relay task 的任意 2xx 修复，I 媒体/流转换，J upload/meta preview，K enum 修复，L 存储迁移。rc.40 测试和终验完成后产生最终 checkpoint。

图片调用链：`host protocol route → TokenAuth/限流 → Pin/Prepare → Distribute → RelayTaskPluginEndpoint → serveTaskPluginImageProtocol → executeTaskSubmission → RelayTaskSubmit/ForcePreConsume → durable Task → 请求内 poll 或后台 poll → terminal CAS/outbox → reconcile → render`。未被认领的模型继续 `Relay → prepareSelectedImageBilling → ImageHelper`。客户端断连只结束等待/写响应，不能取消已接受的上游任务、持久任务或结算。

New API 插件调用链：`ChannelSettings bindings → identity filter/candidate → expected_task_plugin_key → task_plugin_key → TaskAdaptor ctx.upstream.kind=new_api → prefixed native route + Bearer gateway token`；poll/content 同样必须得到 ChannelType/ChannelId，且使用提交时保存的网关 key。

## Patterns：必须保留的 Molii 约定

- `service/channel_select.go:259` 的 `appendUnifiedNativeTaskChannelTypes` **仅**为 Doubao + `openai_video` + 四个 Seedance 2.x 模型桥接 61/64；不是给 Doubao 所有模型放行。`middleware/distributor.go:239` 保留这条 fork-specific 过滤能力。
- rc.39 **Alibaba** 声明 `openai_image`（官方 `plugins/tasks/alibaba/plugin.js:335`）。**Doubao 没有该声明**：它提供 openai_responses（视频+图像）、openai_video（视频限定）和新增 native `/doubao/api/v3/images/generations`（`retainResult:false`）。不得把 Seedream `/v1/images` 送 Doubao 当作官方行为。
- `relay/relay_task.go:321` 起每次重试重建 prepared billing，先模型映射，保留显式 billing override，只有特定缺失价格场景用 mapped-tail；`relay/helper/price.go:78` 的 `ResolveRelayBillingModelName` 不可被官方旧 `resolveBillingModelName` 覆盖。
- `controller/relay.go:763` 完整存储 Molii BillingContext；`:795` 开始 64/62 使用 finalUsageLogOnly、61 保留 StarAI timing。`model/task.go` 中 GroupRatioCaptured（显式零值语义）、GrokVideoBilling、StoredResult 等均不可丢。
- `service/task_polling.go:123` 调 `FinalizeTaskAndEnqueueBillingWithContext`；`model/task_billing_job.go:79` 开始在单事务中做任务 CAS 与唯一 billing job 创建。`model/task_billing_tx.go:40` 保留 lease owner/attempt/expiry fencing、wallet/subscription/token/task/statistics/job 同事务与 commit 后 cache sync。
- `service/task_billing_reconcile.go:152` 已记录异步 perf 样本，`:212/:244` 的 reconcile tests 已检查成功/失败样本。官方新增 RecordTaskResult 不能无条件再记一次。
- `controller/task.go:511`、`service/seedance_task_facts.go:34` 均从 Task.Data 叠加实际 duration/resolution/ratio/input counts；只存预估 BillingContext 不足以替代 Data。
- `model/task.go` 的 `TaskPrivateData.Value()` 用 JSON **string** 存储以兼容 PG simple protocol；新增 ResultDiscarded 必须并入现有 empty-check，不能把 Molii 其他字段从检查中删掉。
- `web/src/context/theme-provider.tsx:74` 当前固定 system，setter/reset 为 no-op；`theme-customization-provider.tsx:92` 当前固定 default customization。`web/src/lib/theme-customization.ts:130` 的默认值由 `SITE_BRAND.defaultFont` 决定，preset anthropic、radius md、scale sm、layout full。

## Risks：P0 兼容阻断与解决边界

### P0-1：把官方终态 direct billing 合进 Molii outbox

官方 rc.39 `finalizeTerminalTask` 在状态 CAS 后直接 settle/refund；Molii 已改为资金意图持久化。覆盖会丢失 worker 崩溃后的可恢复结算；同时保留两条路径会双扣/双退/双记 used quota。必须保留 CAS+job 原子入队及 reconcile 事务，借用上游 usage extraction / task image fields，而不是恢复上游 direct settle。success/failure、batch、timeout、poll failure、channel unavailable 均需走相同 durable boundary。

### P0-2：性能采样双计数或被结算状态污染

新增 terminal RecordTaskResult 和现有 reconcile Record 共存会双计数。推荐把 execution outcome 采样统一移到 durable CAS **commit 后且 won=true**（single/batch/error/timeout 都覆盖），移除 reconcile 的旧样本；结算失败或 review_required 不能改变生成成功率。同时迁移已有 reconcile 性能测试，保留它们的账务日志/排行 token 断言。此方案具有上游同样的 best-effort 进程崩溃漏样边界，不应宣称数据库级 exactly-once 指标。若保留旧采样点，则需实现所有任务路径的互斥，不能仅跳过某个渠道。

### P0-3：预扣默认静默改变

官方 `TrustQuotaUSD=10` 必须按任务约束改为 **0**，且 `ForcePreConsume`、subscription 始终禁止信任绕过。旧 `common.GetTrustQuota()` 为固定 $10，trust=0 是需求指定的 fork 默认，不能以“保留历史”为理由还原它。

另一个独立变化：rc.39 legacy token 预留从 `max(prompt,PreConsumedQuota)+max_tokens` 改成 `prompt*multiplier`；tiered 从估计 completion（缺省8192）改为 `c=0`，只缩放非 request billing。只把 trust 改0 **不能**保住旧预留金额，固定倍率1也不能代替 legacy floor/output 估算。实施须为 Molii GPT Image 2/Grok/native task 单独保留其预留/最终价格快照，未获授权的图片 token 路径不得因通用公式覆盖而归零；通用文本采用新公式或保留旧默认的边界必须在 plan 明确，不可通过调低旧测试期望掩盖变化。实际 settle/fixed-price、管理员显式表达式和价格 overrides 不受预扣倍率改变。

### P0-4：模型同名导致 native Seedance 或图片渠道被插件吞掉

Doubao 新 Seedream 与 Seedance profiles 拓展后，插件认领 happens before channel distribution。61/64 的四个 Seedance 2.x 视频继续 native adaptors；Seedream/1.x/native image/Responses 不得被这些视频专用渠道选中。New API 60 仅允许支持 new_api 的绑定插件；普通 OpenAI 请求不应携带之前重试残留的插件身份。63 仍单插件，不允许复用官方 type61 数值。

### P0-5：丢弃结果或省略列表 Data 破坏 Molii 持久契约

retainResult:false 仅作用于立即终态；异步提交必须留 snapshot 供 poll/retrieve，OpenAI Images 等待中的异步任务亦如此。现有 61/62/64 的 signed playback/COS TTL、Studio request、结果续期、故障回放不可被全局“同步结果不保留”规则覆盖。ResultDiscarded=false 缺省确保历史 JSON 仍可读。

官方普通任务列表 Omit(data) 会丢掉 64 的实际生成事实；必须先持久化受控事实到 PrivateData 并兼容旧数据，或对有需求的 Molii 平台保留安全 facts 查询，再采用普通插件列表 omit。核对 admin/user 列表与 `GetUserTasksForPlatformsPage` 工作台列表，保持角色脱敏与账单详情一致。丢弃结果应在 native query、Responses retrieve、video fetch、artifact projection/content 统一不可取，账务日志仍保留。

### P0-6：LongText 迁移成功不等于可回滚

rc.40 用自定义 `LongText.GormDBDataType`：MySQL longtext、PG/SQLite text；Source 移除旧 type:text，Icon 移除 size:524288。保留 Molii `migration_dialector.go` 的 GORM 比较修正，验证 fresh 与旧表升级后再次 AutoMigrate 不重复 ALTER。需要真实 MySQL：官方 `TestTaskPluginSourceAboveMySQLTextLimitRoundTrips` 名字不能证明真的跑了 MySQL，controller fixture 通常是 SQLite。

回滚到旧二进制可能把 MySQL longtext 再收窄为 text，且旧上传 cap/JS host 不理解新 metadata。只回滚 Docker digest 不能证明数据安全。升级前备份 plugin sources、source hashes、激活版本/disabled factory 配置及 DB；回滚路径应经过兼容迁移验证，避免旧启动 AutoMigrate 收窄。新脚本和旧节点混跑要防止 override 编译失败，利用 incumbent 保留不能当作完全兼容保证。

### P1：主题/品牌、价格 UI 和转换

- rc.40 provider 恢复读取可编辑 localStorage 偏好，会重启 Molii 已禁用的 customization。保持 system/default locked 行为，整合 storage helper/cache 白名单时明确 fork 差异；不能为了通过官方偏好测试恢复任意主题。保留 runtime brand injection，官方 logo 资源可作为上游资源同步，不可改活跃站点 favicon/title/字体来源。
- rc.39 上游 `rsbuild.html.favicon='./public/favicon.ico'` 会与 Molii index 中 templated siteBrand favicon 冲突，不能让两套 icon 标签同时生效。
- 官方新版 task-price display + rc.40 unreachable-enum 分支修复需合并；不改保存的管理员表达式，只改善窄 schema 下展示/编辑。保留 Molii CNY/USD、特殊 group visibility、task log final usage/root info、Grok/Seedance matrix。
- rc.40 任意 2xx 成功需沿用实际 status 供 parseSubmitResponse；201/202 不可当失败并退款，但 204 空内容仍需 provider parser 正常判无效。保持 Molii 超时重试边界与私有 upstream ID。
- relaykit 媒体提升只适用于识别的 content-part 数组；普通 object/array/string 保持旧 stringify。多个 tool outputs 必须连续，媒体统一移到其后 user message；SSE 一个 item done 后的新 delta 必须新 ordinal/ID，不修改已关闭输出。

## Concrete TDD：先 RED 再兼容实现

优先扩展现有合适测试；不为每个中间函数复制一份 fixture。每个版本记录命令、RED 失败断言、GREEN 结果。官方新增已有测试也必须保留。

| 测试位置 | 必测行为与精确期望 |
| --- | --- |
| `service/channel_select_test.go` + `middleware/distributor_test.go` | rc.39 factory generation：四个 Seedance 2.x + openai_video 可选61/64且 platform 仍为61/64；Seedream、1.x、native image 和 Responses 排除61/64；63 dedicated binding；60 双插件 candidate 按 generation 顺序确定，第一次失败/重试不改变计费 provider；普通 GPT Image2/Grok Images 仍原 adaptor。 |
| `controller/plugin_protocol_test.go` + `router/task_plugin_protocol_router_test.go` | 每个 image path 只注册一次；未认领图像 JSON/multipart body 原样 fallback；Alibaba 即时结果、异步 polling、OpenAI error envelope、b64_json 转换（下载失败保留URL）、提交期间断连仍持久化/计费，wait超时504后后台能完成。可用已有 deps 注入避免 sleep。 |
| `controller/relay_task_plugin_test.go` / `controller/task_generic_test.go` / `model/task_starai_test.go` | retain=false immediate：DB Data 无结果、in-memory render完整、ResultDiscarded true；async：Data存在、无 discarded；普通64/61/62与历史行继续 playback/artifact，PrivateData roundtrip 保住 StoredResult/Timing/VideoStudioRequest/GroupRatioCaptured 的0值；用户/管理员task列表实际resolution/ratio/duration与billing详情一致。 |
| `service/task_polling_test.go`、`model/task_billing_job_test.go`、`service/task_billing_reconcile_test.go` | plugin image 与 native Seedance terminal竞争：一个CAS赢家/一个job；入队失败终态更新一起rollback；终态前后wallet/token/statistics分别按预留和最终差额变化；reconcile重跑与lease过期不二次资金变更。生成成功但结算review_required仍记成功样本；普通poll、batch、timeout/failure各只一个样本，reconcile后仍一个；CAS loser不采样。 |
| `relay/helper/price_test.go`、`service/billing_session_test.go`（若当前不存在则扩已有 billing tests）、`controller/billing_option_test.go`、现有Grok/StarAI/64计费测试 | 默认trust=0的钱包余额>$10仍实际预扣；显式阈值启用仅wallet bypass；ForcePreConsume/subscription永不bypass；NaN/Inf/负数/倍率0拒绝save和runtime。GPT Image2、mapped别名、Grok image/video、Seedance固定/零group倍率的预留与final quota与升级前相同。覆盖tiered c=0/input multiplier时只影响reservation，fixed request不被倍增、实际结算不被倍增。 |
| `controller/channel_task_plugin_bind_test.go`、`relay/channel/task/jsplugin/auth_test.go` | NewAPI60单/多绑定、重复/空白/超过32/non-new_api driver拒绝；create/copy/update更改绑定必须有bind权限，未改绑定可编辑普通字段；cascade disable只解绑目标插件，网关仍enabled；submit/poll/content传Bearer提交key，rotation后仍使用原key；multipart同名2文件ref分别指向不同字节。 |
| `model/task_plugin_test.go` + `model/migration_dialector_test.go` | 真SQLite/MySQL/PG fresh和rc.39旧schema升级；>64KiB源码和大型icon完全roundtrip；再次迁移schema recorder无变更，索引/active/version/hash保留；8MiB有效JS允许、8MiB+1拒绝；旧二进制回滚迁移是否收窄需单独验证。 |
| `controller/task_plugin_test.go` | snapshot变metadata-only后 unchanged不重编译；同ID取到hash对应source；changed rows一起publish一个generation；failed source read/compile保留incumbent；disabled/factory fallback/revision保持旧逻辑。保留官方upload/source cap测试。 |
| `plugins/doubao_responses_test.go`、`plugins/alibaba_responses_test.go` | Seedream native与Responses图片正确profile/retain规则，usage actual-image tiers；Seedance per-model resolutions：fast/mini不允许1080p，2.5不允许4k，2.0支持4k；alias→ep-ID仍用client model profile；管理员表达式不被工厂覆盖。Alibaba rc.40 renderer保留上游metadata。 |
| relaykit对应已有tests | Claude tool_result混合text/base64 image，多个tool result顺序；Responses tool-output image/file/audio/video提升后的内容类型与call_id/tool消息连续；普通JSON数组保持stringify；finish_reason后reasoning/text再来新segment、唯一ID、part-added/delta/done成对且最终response包含所有已关闭段。 |
| 前端既有feature tests + `theme-contract.test.ts`、`site-brand.test.ts` | 60多绑定与61/62/64字段完整；upload8MiB/activation和安全const metadata预览；缩窄enum价格仍可编辑/拷贝；task_sync/result_discarded/image_count准确；Seedance/Grok实际金额/人民币单位正确；即使cookie/localStorage放任意preset/font/mode，Molii locked默认不变、系统色彩跟随OS；多站点runtime title/favicon/font仍正确。 |

## 测试与部署/回滚门禁

- 每标签：`GOWORK=off go test ./...`、`GOWORK=off go build ./...`；`cd relaykit && GOWORK=off go test ./... && GOWORK=off go build ./...` 必须独立，root成功不能替代。root实际Go版本为 go.mod 的 **1.26.5**（AGENTS概述仍1.25.1），relaykit为1.25.1。
- 前端用 Bun：`bun run typecheck`、`bun run test`、`bun run build`，相关文件lint、i18n completeness；保持web/dist嵌入条件，不能用占位HTML冒充真实frontend构建。
- 数据库：现有 `.github/workflows/ci.yml:22` 只配 PostgreSQL15+Redis7，没有MySQL；`model/migration_dialector_test.go:59` 可用 `TEST_MYSQL_DSN`/`TEST_POSTGRES_DSN` 跑三方真实测试，未设置会Skip。必须给MySQL门禁供实例，报告真实版本/命令/结果；不能将skip视为通过。既有 `FULL_MIGRATION_POSTGRES_TEST_DSN` 全schema重复迁移及分离log DB路径一起验证。
- 复用 `bash deploy/tests/deploy_test.sh`、`bash deploy/tests/retry_test.sh`、既有 `deploy/migrations/*_test.sh`；验证多Environment的VITE_SITE_*、runtime品牌注入、immutable digest、health URL、env路径隔离、rollback结果和旧镜像metadata保留。`docs-site/scripts/subpath-deployment-contract.test.ts` 与docs workflow契约保护 /docs、健康检查、独立静态发布。
- 本任务无shared push/部署授权：只准备可审查代码、checkpoint和验证记录。未来部署前记录旧image digest、DB备份、plugin activation/source hash、pending/processing/review_required jobs基线；升级后检查既存61/62/63/64渠道、任务polling/COS播放、实际账单、perf不重复。
- `deploy/deploy.sh` 当前rollback只恢复镜像/Compose与health，不恢复DB；rc.40字段收窄风险和新plugin metadata的旧节点兼容必须列入runbook。数据库恢复涉及升级后新账务，不能把“恢复备份”当作无损自动回滚。
