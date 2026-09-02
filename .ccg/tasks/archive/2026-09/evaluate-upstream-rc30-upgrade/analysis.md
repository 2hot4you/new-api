# new-api v1.0.0-rc.30 升级评估

## 结论

当前 Molii `develop` 与上游 `v1.0.0-rc.30` 的真实共同祖先是
`v1.0.0-rc.23`（`0ab02020603d22e5613bc4cf46bfab06f8567769`），不是
`v1.0.0-rc.25`。从共同祖先起，上游有 77 个提交，Molii 有 474 个提交；
其中上游 74 个补丁尚未等价纳入 Molii，只有 3 个补丁等价存在。

不应把 `v1.0.0-rc.30` 直接合并进 `develop`。只读执行
`git merge-tree --write-tree HEAD v1.0.0-rc.30` 得到 72 处冲突，涉及 121 个
可自动合并文件，冲突集中在数据库迁移、计费、异步任务、渠道、API 密钥、
模型定价和用量日志等 Molii 核心模块。

上游从 `rc.27` 起将内置异步任务适配器替换为沙箱 JavaScript 插件。该架构
仍被 `rc.30` 发布说明标记为实验性、不建议用于生产，并要求从 `rc.26` 或更早
版本升级时重新配置全部视频模型价格。Molii 现有 StarAI、Molii Grok、临时素材、
任务轮询、真实用量结算和 COS 结果链路依赖原生 Go 适配器，不能接受上游删除
这些适配器的默认冲突选择。

推荐并行维护两条升级线：

1. `upgrade/rc30-dev`：仅用于开发环境，逐标签合并并完成任务插件兼容层。
2. `develop`：生产候选线只回移已经审计的安全、计费、数据库和稳定性修复，
   在任务插件稳定且 Molii 自定义适配全部通过回归前，不宣称或部署完整 rc.30。

## 分叉与重叠

| 版本区间 | 上游提交 | Patch-ID 不等价 | Patch-ID 等价 | 变更文件 | 与 Molii 重叠 |
| --- | ---: | ---: | ---: | ---: | ---: |
| rc.23 → rc.24 | 8 | 5 | 3 | 35 | 34 |
| rc.24 → rc.25 | 39 | 39 | 0 | 207 | 59 |
| rc.25 → rc.26 | 4 | 4 | 0 | 42 | 14 |
| rc.26 → rc.27 | 7 | 7 | 0 | 352 | 84 |
| rc.27 → rc.28 | 13 | 13 | 0 | 68 | 25 |
| rc.28 → rc.29 | 4 | 4 | 0 | 6 | 3 |
| rc.29 → rc.30 | 2 | 2 | 0 | 4 | 2 |

三个 Patch-ID 等价补丁是：渠道抓取模型分类、Claude/Gemini 原生格式渠道测试、
兑换码额度精度修复。Patch-ID 不等价不代表功能缺失；实施阶段仍须逐项检查 Molii
是否已有改写后的等价实现。

## 逐版本审计

### rc.24

- 增加用户敏感操作限流。
- 修复 HTTP/2 stream reset 时请求体不可重放的问题。
- 改进渠道模型分类、原生格式渠道测试和兑换码精度。
- Molii 已等价包含后三项中的三个补丁，但请求体重放和敏感操作限流仍需审计移植。

### rc.25

- 包含渠道测试安全模式、参数覆盖上下文、Gateway 字段透传控制。
- 包含 Responses 缓存 Token 结算、异步任务退款 `used_quota`、充值并发与钱包
  上限等计费修复。
- 39 个提交均未以补丁等价形式出现在 Molii；与 Molii 有 59 个文件重叠，尤其是
  `service/task_billing.go`、`model/user.go`、`model/token.go`、渠道表单、API 密钥、
  定价和用量日志，必须按提交语义移植，不能整段覆盖。

### rc.26

- 额度钱包升级为支持超过 42 亿的值，并新增启动前数据库类型检查。
- PostgreSQL/MySQL 现有 `users.quota`、`used_quota`、`aff_quota`、`aff_history`
  不是 64 位整数时会拒绝启动，需要先手工迁移。
- 当前 Molii 尚未包含这套检查和钱包上限逻辑，属于高风险数据库/计费迁移。
- 另含 vLLM `thinking_token_budget`、用量日志防浏览器自动填充和 Bun 1.4。

### rc.27

- 352 个文件、约 5.3 万行新增，是本次升级的架构断点。
- 删除上游内置 Go task adaptors，新增 JS 插件运行时、插件路由、插件市场、任务
  artifact、插件计价表达式和大规模前端管理界面。
- 上游明确要求重新配置全部视频模型价格，并标记不建议生产使用。
- Molii 必须保留 `starai`、`moliigrok` 及其原生计费、轮询、临时素材、结果安全
  校验；任务必须继续绑定原始 `channel_id`，不能在轮询时重新选 Key。
- 推荐兼容策略：上游内置供应商使用 JS 插件；Molii 专用渠道暂保留原生 Go
  adapter，通过明确的 channel type 分流。待开发环境稳定后，再逐个评估迁移为插件。

### rc.28

- 新增可选 RSA-OAEP 登录传输加密、任务模型映射别名、SQLite WAL、PostgreSQL
  transaction-pooler 兼容、JSON Valuer 修复、参数校验 400 和 Ali 图片格式修复。
- `RELAY_RESPONSE_HEADER_TIMEOUT` 默认 1800 秒，修复上游迟迟不返回响应头时的
  无界内存增长/OOM；Molii 当前通用 relay 尚无此设置，应优先回移。
- 此版本自身有 MySQL/PostgreSQL 初始化回归，不得作为任何环境的最终部署版本。

### rc.29

- 修复 rc.28 的 GORM core/driver 兼容和 `prefill_groups.name` 唯一约束迁移。
- 后续又被确认仍可能启动失败或重启循环，不得作为最终部署版本。

### rc.30

- 修复用量统计汇总显示零额度的问题；实际计费和余额不受该缺陷影响。
- 增加 PostgreSQL 旧 `tokens.key` 唯一约束的幂等迁移，解决 rc.29 启动问题。
- 当前 Molii 使用 GORM `1.25.2`、MySQL driver `1.4.3`、PostgreSQL driver
  `1.5.2`；rc.30 分别为 `1.25.12`、`1.5.7`、`1.5.9`，依赖升级必须与 rc.29/
  rc.30 的迁移代码一起验证，不能只更新 `go.mod`。

## 主要冲突域

- 数据库：`model/main.go`、`model/token.go`、`model/prefill_group.go`、GORM 依赖。
- 计费：`model/pricing.go`、`service/task_billing.go`、tiered billing、pricing UI。
- 异步任务：`controller/task.go`、`model/task.go`、`service/task_polling.go`、
  `relay/channel/task/**`、视频路由与代理。
- Molii 渠道：StarAI 四 Key/四渠道模型路由、Molii Grok、临时素材、COS、结果域名
  安全校验和原始渠道轮询。
- API 密钥：有序跨分组路由和自定义前端。
- 用量与模型广场：GPT Image 2 详情、图像生成记录、模型元数据、动态定价。

## 数据库上线前置条件

1. 用与生产相同版本的 PostgreSQL 克隆库完成演练。
2. 完整逻辑备份和 schema-only 备份，记录恢复命令与恢复耗时。
3. 查询 `users` 四个额度列类型，必要时在维护窗口改为 `BIGINT`。
4. 检查 `tokens.key` 和 `prefill_groups.name` 的历史唯一约束名称与定义。
5. 在克隆库上执行最终 rc.30 候选二进制一次，确认迁移幂等；重启第二次仍应成功。
6. 不设置 `SKIP_64BIT_QUOTA_SCHEMA_CHECK=true` 规避生产问题。
7. rc.28、rc.29 只作为 Git 集成检查点，不启动对应构建。

## 验证门槛

- 后端全量 Go 测试、race-sensitive 计费/额度测试和 relaykit 独立测试。
- 前端类型检查、单测、构建、i18n 同步检查。
- PostgreSQL 迁移首次启动和第二次幂等启动测试。
- API Key 单分组、有序跨分组、失败转移和更新/创建测试。
- StarAI 2.0/fast/mini/2.5 四渠道提交、查询、结算、临时素材和结果 URL 测试。
- Molii Grok、GPT Image 2/COS、图像下载、用量日志和生成记录回归。
- 文本、Responses、Claude、Gemini、图片、视频端点的鉴权、计费和错误码冒烟。
- 视频模型价格导出、插件迁移后重新配置、价格对照和小额真实请求核账。

## 回滚原则

- 应用回滚与数据库回滚分开设计；旧二进制是否可读取迁移后 schema 必须在克隆库验证。
- 新版本首次启动前停止写入或进入维护模式，保存数据库快照。
- 任何余额、预扣、最终结算、退款或任务 `channel_id` 不一致都视为阻断上线。
- 开发环境至少稳定观察 24–48 小时，再讨论生产灰度。
