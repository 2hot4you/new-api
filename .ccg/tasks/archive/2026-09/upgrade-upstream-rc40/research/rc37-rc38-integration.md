# rc.37 → rc.38 顺序集成研究

## 基线与证据

- 检查对象：`origin/develop`（研究时 HEAD `3f8d06115`）、官方 `v1.0.0-rc.36/37/38`；同时确认 rc.40 未继续占用 64 以上渠道编号。读取了根 `AGENTS.md` 与本任务 requirements；本工作树不存在 `.ccg/spec/`。
- rc.36→37：433 文件、+41539/-9307；rc.37→38：349 文件、+28493/-3860。不能把这次升级视为简单版本号更新。
- `git merge-tree --write-tree --name-only origin/develop v1.0.0-rc.37` 实测 **38 个冲突文件**。直接累计合并 rc.38 为 **62 个冲突文件**；后者只用来定位风险，**不是**已解决 rc.37 后的真实冲突数。人为指定 rc.37 merge-base 的投影也包含“当前分支尚无 rc.37 文件”的假冲突，不能作为执行清单。
- 未修改业务代码、未运行迁移或测试；以下为静态研究和无工作区修改的 merge-tree 结果。

## 文件分组与依赖顺序

| 顺序 | 文件范围 | 实施要点 |
|---|---|---|
| 37-A | `relaykit/dto/{billing_usage,openai_image,openai_response,usage_merge,legacy_dalle_image}.go`；`pkg/billingexpr/{compile,run,settle,types,fixed}.go` | 先落 usage、图片数量/缓存、fixed() 表达式契约，保持 relaykit 独立模块。读 `pkg/billingexpr/expr.md` 后实施计费。 |
| 37-B | `model/{option_primary_key_migration,model_pricing_config,model_pricing_conversion,legacy_dalle_pricing,option,main,pricing}.go`；`setting/billing_setting/{builtin_billing,tiered_billing}.go`；`setting/ratio_setting/{model_ratio,cache_ratio}.go` | 合并唯一键修复、原子价格写入/转换和新默认表达式；保留 Molii 配置归一化、币种/市场元数据和管理员价格优先级。 |
| 37-C | `relay/{image_handler,relay_task}.go`；`relay/channel/{openai/relay_image,task/jsplugin/adaptor}.go`；`relay/helper/{price,billing_expr_request,valid_request}.go`；`service/{image_billing,billing_session,task_billing,task_polling,text_quota,tiered_settle}.go` | 接入新计费输入/输出，保留 Molii 模型映射、敏感信息屏蔽、COS 预览、异步 outbox 和终态结算。 |
| 37-D | `pkg/jsplugin/{registry,routing,engine,json_state}.go`；`middleware/{task_plugin,distributor}.go`；`service/channel_select.go`；`dto/{task,channel_constraints}.go`；`plugins/tasks/{alibaba,sora}/plugin.js` | 插件 generation、模型级 usage schema、同步/流式提交、筛选与能力声明一起集成；不可破坏 Molii 原任务渠道重试/绑定。 |
| 37-E | `model/{passkey,passkey_option}.go`；`setting/system_setting/passkey.go`；`service/passkey/*`；`controller/{passkey,option,secure_verification,login_verification}.go`；`web/src/features/{auth/passkey,system-settings/auth}/` | RP ID 绑定、兼容域名与安全验证是一条完整链路；前后端一起落地。 |
| 37-F | `web/src/features/channels/{constants,components/drawers/*,lib/channel-configuration,lib/channel-plugin-extensions,hooks/use-channel-model-discovery}.ts*`；`features/{pricing,model-pricing,system-settings/models}/` | 上游重写渠道编辑器（单文件差异约 7071 行），拆出 provider picker/configuration；迁入 Molii Provider、Seedance 表单、市场排序/币种/自定义价格展示。 |
| 38-A | `constant/channel.go`；`common/{api_type,endpoint_type,advanced_custom_presets}.go`；`model/channel.go`；`controller/channel_inference.go`；`router/channel-router.go`；前端 `channels/{constants,lib/channel-type-config,lib/channel-form,components/*inference*}` | **先**确定持久化编号，再接 vLLM/SGLang 预设、模型发现、状态查询和前端表单。 |
| 38-B | `model/{request_policy,option}.go`；`service/{request_policy,channel_select,channel_affinity,relay_error}.go`；`controller/{request_policy,relay}.go`；`middleware/{auth,distributor,model-rate-limit}.go` | 完整有效策略快照、自动/严格会话、重试、限流预留和路由决策日志。不能只拿设置 UI。 |
| 38-C | `controller/responses_websocket.go`；`relay/{responses_websocket,responses_request,request_billing,responses_handler}.go`；`service/{responses_usage,ws_close,quota}.go`；`pkg/wsmanager/*`；`router/relay-router.go`；`main.go`；`relaykit/dto/channel_settings.go` | 复用 38-B 的 HTTP 路由/策略/计费，接 WebSocket 每请求鉴权、限流、计费、错误关联、连接撤销。 |
| 38-D | `model/{migration_dialector,prefill_group_migration}.go`；`service/authz/adapter.go`；`model/{system_task,user,account_security,login_verification}.go`；`oauth/{github,provider}.go` | 真实数据库稳定性、旧约束名兼容、历史授权 scope 按 deny 加载、GitHub 历史绑定迁移证据。 |
| 38-E | `pkg/perf_metrics/*`；`relay/common/{stream_status,response_model}.go`；`controller/perf_metrics.go`；`web/src/features/{system-settings/request-policies,system-info,performance-metrics,usage-logs,channels}/` | 请求边界 outcome、流式异常分类、任务历史清理和 UI；保留 Molii 视频性能指标/消费日志。 |

rc.37 完成时先提交独立 merge/checkpoint 并运行针对性验证，再真实 merge rc.38、重新列冲突。每阶段禁止同时由两个实施者编辑 `model/option.go`、`service/quota.go`、`controller/relay.go` 或渠道编辑器。

## P0：渠道编号是持久化协议

现有契约位于 `origin/develop:constant/channel.go:61-64`、前端 `constants.ts:25-28`：

| 数字 | Molii 已落库身份 | 官方 rc.38 身份 | 最终决定 |
|---|---|---|---|
| 61 | StarAI | Task Plugin | 保留 StarAI |
| 62 | Molii Grok | vLLM | 保留 Grok |
| 63 | Task Plugin | SGLang | 保留 Task Plugin |
| 64 | ByteDance Seedance | 无 | 保留 Seedance |
| 65 | 未使用 | 无 | 建议 vLLM |
| 66 | 未使用 | 无 | 建议 SGLang |

- **不做把 61→63 或 62/63→新类型的 SQL 迁移**：这些记录原本就是 Molii 渠道，不是官方渠道。显式定义 vLLM=65、SGLang=66；`ChannelTypeDummy` 置于 66 之后；`ChannelBaseURLs` 补足索引，原 61 的 StarAI URL、62 的 Grok URL 不变。
- 前后端名称、描述、显示顺序、密钥提示、图标、能力集合、表单默认值全部用新常量。官方 `web/.../constants.ts` 的 **裸数字 61/62/63** 出现在名称、描述、顺序和提示中，不能只改三个 `export const`。
- `common/api_type.go:78` 的 AdvancedCustom case 加 65/66，同时保留 Molii Grok case（基线 :84）和 TaskPlugin 禁止降级 OpenAI（:88）。`relay/relay_adaptor.go:163-170` 解析任务 platform 数字字符串，必须维持 `61/62/64` 对应原任务适配器；63 仍进入插件身份解析路径。
- 65/66 使用 `APITypeAdvancedCustom`，不新增冲突的 API type；同步 `IsAdvancedCustomChannel`、`GetAdvancedCustomPreset`、endpoint 推导、channel settings 验证、inference 状态路由。检查 `streamSupportedChannels` 是否满足上游 OpenAI-compatible stream usage 支持。

## P0：计费、预览与任务结算

- rc.37 新增 GPT Image 2/2.5 默认表达式 `p*5 + cr*1.25 + img*8 + img_cr*2 + c*30`；这里只报告仓库公式，不对实时供应商价格作外部验证。Molii 在 `setting/billing_setting/tiered_billing.go:57-80` 已保证显式模式、ModelRatio/ModelPrice 覆盖新内置值，这段有冲突，必须保留。不能让已有管理员价格在启动后默默切换为 tiered_expr。
- 新 `service/image_billing.go` 每次尝试按最终有效数量重算 reserve，`n`/prompt_extend 跨重试必须覆盖而非累乘；token-only 图片表达式不得自动乘数量。rc.38 改为按有效图片 payload 计数，并把 `output_tokens_details.image_tokens` 进入 `img_o`。合并需防止重复计算图片输出 token 或 count。
- Molii `relay/image_handler.go:30-48` 已冻结日志快照、避免重复模型映射、禁止 Grok 透传；rc.37 的准备图片计费必须插在有效模型/override 已确定的位置，并维持这些边界。
- `relay/channel/openai/relay_image.go` 的非流式、SSE、JSON-as-stream 三条路径都有 GPT Image 2 COS 预览持久化、完成时刻、OutputCount；上游 usage/count 修正不能覆盖它们。持久化失败只降级预览，不导致再次计费或泄露 upstream/base64。
- `service/task_billing.go` 新增 `EvaluateTaskCompletionUsage` 明确只计算、不移动资金。Molii 的 `task_billing_job`/事务/outbox、终态单日志、Seedance 人民币价格快照、Grok 最终用量、失败退款仍是资金副作用入口。同步完成和轮询完成必须各经过原有持久化屏障，不直接套官方立即扣费路径。
- `model/pricing.go` 的 `BillingCurrency`、`VideoPricing`、`MoliiGrokPricing`、市场元数据与上游 `BillingPluginVariants`/usage schema 应并存。前端不能把人民币价格当 USD 再换算，也不能丢弃 Molii 的分组/排序和输出类型。

## 数据库、Passkey 和请求策略风险

- rc.37 `migrateOptionPrimaryKey` 在 `AutoMigrate` 前检测 `options.key` 唯一性；非唯一表重建为临时表再交换，旧数据保留到 `options_legacy_<timestamp>`。重复 key 采用读取顺序最后一行（无稳定 ORDER BY），冲突值应在升级前审计；不允许把“迁移完成”理解成自动选中了业务正确值。`model/main.go` 对迁移错误只 SysError 后继续，健康检查通过不能证明修复成功。
- `model/model_pricing_config.go:540-569` 使用行锁、OnConflict DoNothing 与按 key Update 替代危险整行 Save；先保证唯一键再验价格更新不抹掉其他选项。`model/option.go` 合并要保留 Molii `normalizeOptionValue`（品牌/group_metadata）、COS 初始化、StarAI 价格缓存失效，并引入 passkey 集中加载和 request policy 原子快照；混合 bulk 更新时不要丢掉归一化结果或造成锁反序。
- rc.37 增加 nullable `passkey_credentials.rp_id`；历史 NULL/空凭据仅在**验证成功**后绑定 RP，已绑定的不同 RP 不可覆盖。主域变化会保留旧 RP；移除兼容域有 preview/confirmation 计数流程。Molii 多站点独立 ServerAddress、Origin、HTTPS 及品牌配置不能串站；rc.38 的已保存兼容域恢复修复要一起保留。
- rc.38 migration dialector 修复 MySQL decimal 默认值等价、Postgres bpchar 和历史唯一约束名；prefill 迁移按定义替换全局 name 唯一性，并保留复合/表达式/partial 索引。必须在真实三种数据库检查 fresh + rc.36 升级 + 两次启动；不要仅凭 SQLite mocks 声称兼容。
- rc.38 `service/authz/adapter.go` 对历史 own/other scope 转为内存 deny；这是预期安全收紧但可能让旧管理员失去具体权限。升级审计存储的 V4/V5，不可为“兼容”静默扩权。
- WebSocket `GET /v1/responses` 在 `router/relay-router.go:76-80` 先鉴权，选渠道延迟至首个 response.create；每个事件重新走普通限流/鉴权 runner。连接后 token 停用、限额耗尽、model access 变化不能绕过；禁用渠道需 `wsmanager.StartSubscriber` 跨实例关闭。上游 `PrepareRequestBilling/RefundFailedRequestBilling` 必须与 Molii Billing 会话、日志和预扣回滚保持一致。

## 冲突处理模式

- rc.37 后端实际冲突：`controller/billing_option_test.go`、`model/{option,pricing}.go`、`relay/channel/openai/{relay_image.go,image_stream_test.go}`、`relay/{image_handler,relay_task,relay_task_test}.go`、`service/{channel_select,task_billing}.go`、`setting/billing_setting/tiered_billing.go`。
- 前端实际冲突包括 channels drawer/constants、pricing 多组件和动态公式、model-pricing panel、keys、models、usage-logs、品牌 header/system-brand、use-system-config、main.tsx、pricing routes 和 locale。使用上游重构后的组成结构迁回 Molii 功能；**不能对整文件 ours/theirs**。
- `web/src/features/models/components/dialogs/sync-wizard-dialog.tsx` 为 Molii 已删除、官方修改的 modify/delete 冲突；先检查 Molii 的替代入口，避免复活被移除流程。主入口的 provider/error toast 改动须保留运行时 branding 初始化。
- rc.38 特别新增的累计风险：`common/endpoint_type.go`、`constant/channel.go`、`controller/{relay,perf_metrics}.go`、`main.go`、`pkg/perf_metrics/*`、`relay/channel/openai/relay_responses.go`、`service/quota.go` 及渠道表单类型配置。即便 clean merge，也检查 Molii stream privacy、视频指标和 outbox 语义。

## 先红后绿的具体回归用例

1. **落库编号→行为**：在现有 adaptor registration/relay tests 建立 type=61/62/63/64 的渠道记录和旧 task platform；选择/重试/轮询仍命中 StarAI/Grok/插件/Seedance。新增 65/66 分别拿到 vLLM/SGLang route preset；模型发现与 inference status 不落入 Grok/插件路径。测试 API/行为而不是只断言常量。
2. **前端保存契约**：扩展渠道 `channel-configuration.test.tsx`/现有 Molii drawer 测试，加载四种旧类型后保存，payload type 不变；选择两种新 Provider 保存 65/66，具有各自默认路由和提示；63 仍显示插件扩展、64 保留 Seedance 模型/密钥控件。
3. **管理员价格不漂移**：扩展 `setting/billing_setting/model_tiers_test.go` 和合适的 billing option 既有测试：已有 GPT Image 2 ModelPrice/ModelRatio/显式表达式分别保持原有效模式与一次请求实际 quota；新空配置才采用默认表达式。同时请求 pricing API 验 CNY Seedance、Grok、plugin variants 字段共存。
4. **图片 reserve→settle**：扩展 `relay/image_handler_mapping_test.go`、`image_stream_test.go` 中最合适现有文件：映射只执行一次；n 参数 override 后预扣正确；Ali 失败转普通渠道不残留 prompt_extend；有效两张图+空 data 项只收两张；img_o 与缓存输入准确；非流式/SSE/JSON fallback 仍生成 COS 预览且只结算一次。
5. **任务资金幂等性**：扩展现有 `relay_task_retry_billing_test.go`、`service/task_billing_reconcile_test.go`：同步完成、新插件实际 usage、重复轮询、持久化失败重试、跨渠道重试，各自最终余额与单条消费/退款日志一致；旧 Seedance/Grok 冻结价格不受管理员后来改价影响。
6. **迁移真实矩阵**：沿用 `model/postgres_migration_integration_test.go`（`FULL_MIGRATION_POSTGRES_TEST_DSN`）并补 MySQL/SQLite同等路径；无 PK/重复 options、定制品牌/COS/价格、旧 NULL rp_id、任意旧 prefill unique 名、任务 outbox/文件/市场表全部入 fixture。升级两次，验证数据/唯一性/索引、价格单项更新不清空他项；额外运行上游 `TestMigrationSchemaStability`，提供 `TEST_MYSQL_DSN`/`TEST_POSTGRES_DSN`，确认没有 SKIP。
7. **Passkey**：沿用 controller/passkey_test + domain-verification UI：历史 credential 在原域认证绑定成功，错误 RP/Origin 拒绝且不绑定；跨站重放拒绝；旧域保留后仍可登录；preview 不写数据库；无确认移除有凭据域失败；并发域变更和 assertion 不产生不一致。
8. **WS/Policy**：沿用上游 controller/relay responses_websocket tests：同一连接第二请求撤销 token/耗尽余额/更换模型权限被阻止；strict affinity 不偷偷换渠道；失败只退一次；中断终态 usage 用 accumulator 估算并结算一次；stream_id 对应错误；禁用渠道关闭连接；Molii image/task HTTP 路由不被新通用策略误分流。

## 验证与部署交接

- 每版本 checkpoint：根 `go test ./...` 与构建；`cd relaykit && GOWORK=off go test ./... && GOWORK=off go build ./...`；`cd web && bun install --frozen-lockfile && bun run typecheck && bun run test && bun run build`；后端并发敏感范围执行 CI 已有 `go test -race ./common ./middleware ./service ./controller ./relay ./router -count=1`。执行新增行为测试的第一次失败应针对缺失兼容逻辑，不应只是编译错误。
- 现有 `.github/workflows/ci.yml:20-39` 只明确配置 PostgreSQL 15；不能把“CI 绿”当作完整三库验证。rc.36→38 没有 go.mod/go.sum/relaykit go.mod 的变化，但 migration dialector 本身仍触发三库矩阵要求。
- 跑 `deploy/tests/deploy_test.sh`、`deploy/tests/retry_test.sh`、两项 deploy Python tests 和受影响迁移脚本测试；保留 `.github/workflows/deploy.yml`、所有 `deploy/env/*` 和 runtime branding/站点映射。代码集成不授权实际部署或推送共享分支。
- 上线前备份 PostgreSQL 与有效 options/渠道编号快照，逐站检查 RP/Origin；迁移有表交换/约束变化，`deploy/deploy.sh` 的旧镜像回滚并不回滚数据库。反向代理必须转发 GET /v1/responses 的 Upgrade/Connection 并允许足够流式空闲时间；多副本保留原 Redis 配置以支持 wsmanager 撤销传播。
