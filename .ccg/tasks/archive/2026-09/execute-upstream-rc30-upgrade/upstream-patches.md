# 上游补丁处置台账

状态说明：`equivalent` 表示 Molii 已有经过测试的改写实现；`adopt` 表示需要纳入；
`rewrite` 表示需结合 Molii 定制重写；`reject` 表示明确不适用于本 fork。

## v1.0.0-rc.24

| Commit | 状态 | 依据 |
| --- | --- | --- |
| `d6b5ce99d` HTTP/2 请求体重放 | equivalent | Molii 已有 `ReplayableBody`、独立 reader、`GetBody`、禁止跟随重定向及 HTTP/2 回归测试。 |
| `ea4f02101` replay metadata 重构 | equivalent | Molii 将 replay metadata 封装在 body 类型中，避免跨渠道共享陈旧 metadata。 |
| `0cd9dc85e` 用户并发更新加固 | equivalent | 已有 `UpdateUserAccessToken`、邀请额度原子更新、用户更新排除并发计费与 token 字段及测试。 |
| `c9bc03864` 渠道抓取模型分类 | equivalent | Git patch-ID 等价。 |
| `b941253ae` Claude/Gemini 原生测试格式 | equivalent | Git patch-ID 等价。 |
| `1da23d6b3` 用户敏感操作限流 | equivalent | 已有按用户和 scope 的 `UserCriticalRateLimit`，access-token 与 aff-transfer 路由已启用并有 Redis key 测试。 |
| `e926e5cac` 兑换码额度精度 | equivalent | Git patch-ID 等价。 |
| `5c3abffe8` GitCode 发布同步 | reject | Molii 使用独立 origin/deploy workflow，不同步 QuantumNous 的 GitCode release。 |

定向验证：

```text
go test ./common -run 'Test.*BodyStorage' -count=1
go test ./relay/common -run 'TestNewOutboundJSONBody' -count=1
go test ./relay/channel -run 'TestApplyUpstream|TestDoTaskApiRequest|TestUpstreamGetBody|TestDoRequest_DoesNotFollowRedirects' -count=1
go test ./middleware -run 'TestUserCriticalRateLimit' -count=1
go test ./model -run 'TestUserUpdate|TestUpdateUserAccessToken' -count=1
```

结果：全部通过。

## v1.0.0-rc.25

| 变更域 | 状态 | 处置与依据 |
| --- | --- | --- |
| 用户与令牌配额原子预扣、Redis mutation fence | adopt + rewrite | 纳入上游条件更新、缓存冷启动和 mutation fence；保留 Molii 默认令牌、轮换、跨分组与任务计费 token 逻辑。 |
| 异步任务失败退款与差额结算 | adopt + rewrite | 普通任务失败时回减累计用量；差额结算只调整用量、不重复累计请求次数；StarAI/Molii Grok 继续使用终态一次性记录。 |
| Advanced Custom 余额查询 | adopt | 纳入自定义余额响应解析、大小限制、原始响应展示及查询参数敏感值清理。 |
| 渠道状态并发更新保护 | adopt + rewrite | 纳入状态字段白名单与按渠道轮询锁，保留 Molii 渠道字段和 Molii Grok 固定上游配置。 |
| Relay 请求转换、reasoning effort、Claude schema | adopt | 接收上游转换与回归测试；Molii 自定义渠道测试全部继续通过。 |
| 前端 Vitest 基线 | adopt + rewrite | 接收 Vitest、Testing Library 和 CI 入口；将 Molii 的 `web/src` 测试迁移到 Vitest，并为 Emoji Mart、VChart 及 DOM 动画 API 增加测试专用替身。 |
| 动态计费规则命中明细 | adopt + rewrite | 在 Molii 用量详情的动态计费分解中展示 `request_rules`，保留现有日志与计费卡片。 |
| 定价搜索防抖 | adopt + rewrite | 纳入 200ms 防抖，保留 Molii 推荐排序与目录行为。 |
| compact model suffix 删除 | reject | Molii 的模型映射和 Imagine 能力校验仍依赖 compact suffix，保留现有实现及测试。 |

验证结果：

```text
make test                                  PASS
bun run typecheck                          PASS
bun run test -- --reporter=dot             PASS (120 files, 565 tests)
bun run build                              PASS
git diff --check                           PASS
```

CCG 规定的 antigravity/Claude 外部双模型审查工具在当前主机不可用：
`~/.claude/bin/codeagent-wrapper` 不存在，`PATH` 中也无 `codeagent-wrapper`。
已通过完整后端测试、完整前端测试、类型检查和生产构建进行替代验证；后续版本继续保留该阻断记录。

## v1.0.0-rc.26

| 变更域 | 状态 | 处置与依据 |
| --- | --- | --- |
| 钱包额度扩展到 JavaScript-safe 64 位范围 | adopt | 纳入 `MaxWalletQuota`、充值/兑换/管理员调额边界检查与原子余额上限保护；单请求费用仍保持 32 位饱和上限。 |
| 旧 32 位用户额度 schema 启动阻断 | adopt | PostgreSQL/MySQL 启动时先检查 `users` 四个额度列，未迁移则拒绝启动；不使用跳过开关作为部署方案。迁移步骤记录于 `migration-rc26.md`。 |
| 批量额度累加溢出保护 | adopt | 纳入批量缓存增量的机器字长溢出钳制和告警。 |
| 充值、兑换码、余额购买订阅 | adopt | 全部改走 64 位钱包严格转换及数据库条件更新，避免并发回调越过钱包上限。 |
| 单请求费用与媒体 token 换算 | adopt | 继续使用 32 位饱和助手，补齐渠道测试、违规费用、OpenRouter cache、图像 token 等裸转换路径。 |
| 模型限流容量计算 | adopt | 时长和 count×duration 使用 64 位溢出保护。 |
| vLLM `thinking_token_budget` | adopt | 纳入通用 OpenAI 请求透传字段。 |
| 日志筛选框密码管理器误填 | adopt | 使用文本输入配合 `-webkit-text-security`，并关闭自动填充。 |
| Bun 1.4.0 | adopt + rewrite | CI/Electron/Docker 统一固定 1.4.0；Docker 保留 Molii 的 `APP_VERSION`、VCS label 与构建参数。 |

验证结果：

```text
make test                                  PASS
bun run typecheck                          PASS
bun run test -- --reporter=dot             PASS (120 files, 565 tests)
git diff --check                           PASS
```

## v1.0.0-rc.27

| 变更域 | 状态 | 处置与依据 |
| --- | --- | --- |
| 异步任务 JavaScript 插件系统 | adopt + rewrite | 纳入沙箱、插件注册表、内置任务插件、插件 API 与管理界面；Molii StarAI 和 Molii Grok 继续使用原生 Go 适配器，形成显式双栈。 |
| 任务渠道类型编号 | rewrite | 保留 StarAI=61、MoliiGrok=62，TaskPlugin 使用 63、Dummy 顺延为 64，避免破坏 Molii 现有渠道数据。 |
| 异步任务计费与批量轮询 | adopt + rewrite | 纳入插件用量 schema、批量 fetch 与持久化结算作业；保留 Molii 终态真实结算、退款、COS 资源和任务日志字段。 |
| 任务产物访问 | adopt + rewrite | 纳入上游任务 artifact 权限与存储抽象，同时保留 Molii 已有签名代理和媒体安全边界。 |
| 渠道与模型定价前端 | adopt + rewrite | 纳入任务插件绑定、usage schema、矩阵定价与任务筛选；保留 Molii 动态定价可读规则、人民币显示、推荐目录与视频/Grok 元数据。 |
| 用量日志插件诊断 | adopt + rewrite | 纳入管理员插件信息、root 诊断、usage facts 与任务产物；保留 StarAI、Molii Grok、GPT Image 2 专属预览和安全日志降级。 |
| 日志敏感字段 | rewrite | `root_info` 仅保留在受权限控制的任务详情中，不写入普通结算日志，避免泄漏上游任务 ID。 |
| 国际化与临时素材路由 | rewrite | 以 Molii 丰富语言集为基准补齐插件键，并重新生成路由树，保留 `/temporary-assets`。 |

验证结果：

```text
go test ./...                              PASS
bun run typecheck                          PASS
bun run test                               PASS (144 files, 783 tests)
bun run build:check                        PASS
bun run i18n:check                         PASS
bun run format:plugins:check               PASS
bun run lint:plugins                       PASS (仅上游 preserve-caught-error warnings)
git diff --check                           PASS
```

`bun run format:check` 与 `bun run lint` 在 rc.27 工具链下会报告大量 Molii 基线文件及
部分上游文件的既有格式/规则差异；未在本检查点做跨模块机械重写。类型检查、完整测试和
生产构建均已通过，最终 rc.30 检查点继续评估并记录该工具链基线。
