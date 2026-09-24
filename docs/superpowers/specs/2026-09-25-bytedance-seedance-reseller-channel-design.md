# ByteDance Seedance 代理渠道设计

## 1. 背景与目标

Molii 当前通过 `Molii Volcengine Imagine API` 渠道直连 StarAI，并提供 Seedance 视频、临时素材、异步任务查询、视频内容代理和最终结算能力。代理商部署独立 new-api 实例后，不应获得 StarAI Key，也不应把 Molii 的直连渠道当作 StarAI 使用。

本设计新增原生渠道类型 `ByteDance Seedance`。代理商实例只配置 Molii 上游 Base URL（例如 `https://aigc.ixiaozu.cn`）与该实例专属 API Key，即可完整使用 Seedance 与临时素材协议。Molii 继续持有 StarAI 渠道和凭据。

首版仅覆盖：

- `POST /v1/assets`
- `GET /v1/assets/{asset_id}`
- `DELETE /v1/assets/{asset_id}`
- `asset://...` 图片、视频和音频引用
- `POST /v1/video/generations`
- `GET /v1/video/generations/{task_id}`
- `GET|HEAD /v1/videos/{task_id}/content`
- 授权模型自动同步、异步轮询、错误透传和独立结算

文本、Claude、Gemini、图片生成和 Grok 协议不在首版范围内。

## 2. 已选方案

新增独立原生渠道类型：

```text
ChannelTypeByteDanceSeedance
后台名称：ByteDance Seedance
```

渠道与现有 `ChannelTypeStarAI` 分开注册。两者共享 Seedance 请求结构、校验、响应解析和状态映射，但不共享渠道语义：

- StarAI 渠道负责直连 StarAI、Molii COS 持久化、StarAI 时间字段和 Molii 批发结算。
- ByteDance Seedance 渠道负责连接 Molii 公共 API、授权模型同步、Molii 任务轮询和鉴权内容代理。

未采用：

- 在现有 StarAI 渠道增加“直连/分销”模式：大量 `ChannelTypeStarAI` 特殊分支需要识别模式，容易错误触发 COS 与结算逻辑。
- 通过 JS 任务插件实现：不能可靠覆盖素材所有权、自动模型同步、原子结算和鉴权内容代理。

## 3. 部署与身份边界

调用链为：

```text
终端用户
  → 代理商 new-api
  → ByteDance Seedance 渠道
  → aigc.ixiaozu.cn
  → Molii Volcengine Imagine API 渠道
  → StarAI
```

每个代理商部署实例使用一个独立 Molii 账户和专属 API Key。多个实例不得共用同一上游 Key。Molii 按实例独立管理模型授权、额度、计费、素材归属、审计、限流与吊销。

代理商实例继续负责其内部终端用户的身份、余额、Key 和素材权限。Molii 不接收代理商终端用户的用户名或密码。

协议不限制代理层数，不增加专用跳数头；其行为与其他 new-api 渠道一致。仅拒绝将 Base URL 直接配置成本实例自身的明显循环配置。

## 4. 渠道配置与模型同步

渠道表单包含：

- Base URL，必填；
- API Key，必填；
- 分组、优先级、权重和状态；
- 上游模型检查与自动同步开关；
- 测试连接并获取模型；
- 手动刷新模型；
- 上次成功时间、同步状态和失败原因；
- 上游授权模型与本地启用模型。

模型列表通过下列请求获取：

```http
GET {BaseURL}/v1/models
Authorization: Bearer {实例专属 Key}
```

同步规则：

- 仅接受上游 Key 授权的 `doubao-seedance-*` 模型；
- 只有本地适配器已有明确能力定义的模型才能启用；
- 管理员可以在本地禁用部分授权模型；
- 上游撤销授权后，从渠道可用模型中移除；
- 同步失败不清空旧列表；
- 复用已有上游模型检测、手动审核和自动应用机制；
- 不同步价格、倍率、币种、渠道 ID或批发成本。

渠道测试不得创建收费任务。测试只验证 Base URL、Key、`/v1/models` 响应和至少一个受支持的 Seedance 模型。

## 5. 临时素材链路

### 5.1 创建

```text
用户 → 代理商 POST /v1/assets
     → Molii POST /v1/assets
     → StarAI POST /v1/assets
```

上游返回的 `asset-...` ID逐层原样返回，不生成 `asset-molii-*` 等额外 ID。每一层仍保存自己的本地授权绑定。

代理商绑定至少包含：

- asset ID和上游 asset ID；
- 本地用户 ID、创建 Token ID；
- 渠道 ID；
- 素材类型、名称、来源类型和来源 URL/COS key；
- 状态、安全错误；
- `created_at`、`verified_at`、`expires_at`。

本地有效期不能超过 Molii 返回的上游有效期。当前上游素材有效期为 168 小时。

### 5.2 用户和实例隔离

代理商实例先按本地用户 ID校验素材绑定。用户 B即使知道用户 A的 asset ID也不能查询或用于生成。

Molii 再按代理商实例专属 Key 所属用户校验自己的绑定。代理商实例 A不能使用实例 B创建的素材。

同一用户的不同 API Key可以复用该用户素材。渠道 Key轮换后：

- 新 Key属于同一个 Molii 账户时，已有素材继续可用；
- 新 Key属于不同 Molii 账户时，已有素材不可用。

### 5.3 状态与删除

GET 查询逐层刷新 `status`、`asset_type`、时间字段和安全错误。素材生成请求逐层验证归属、状态和有效期后，保持 `asset://asset-...` 不变发送到 StarAI。

DELETE 保持现有语义：删除当前实例的绑定与可见性；上游素材按其生命周期自然过期。

### 5.4 本地文件上传

公网 URL可直接提交。代理商本地文件上传继续依赖其自身 COS：本地 COS 生成可读取 URL后，再将 URL提交给 Molii。未配置 COS时仍可使用公网 URL，但不提供本地文件上传。本次不新增上游代传文件协议。

首版不要求同一个路由分组同时混用 ByteDance Seedance 与直连 StarAI 素材渠道。

## 6. 视频任务与 ID 分层

任务 ID分层如下：

```text
代理商公开 task_id
  → Molii 公开 task_id
    → StarAI upstream_task_id
```

代理商提交 `/v1/video/generations` 后：

- 创建并返回代理商自己的公开 task ID；
- 把 Molii 返回的公开 task ID保存为私有上游任务 ID；
- 不保存或返回 Molii 响应中可能出现的 StarAI 原始任务 ID；
- 使用代理商 task ID查询本地任务；
- 后台只轮询 Molii 公开任务。

代理商管理员可以查看代理商任务 ID、Molii 上游任务 ID和请求 ID。StarAI 原始任务 ID仅在 Molii 超级管理员根级诊断中展示。

轮询解析：状态、进度、提交/开始/完成时间、分辨率、实际时长、输入媒体数量、实际 Token 和安全失败信息。原始 StarAI 响应不会存入代理商任务数据。

## 7. 视频内容代理

代理商不重复保存生成视频。终端用户请求：

```http
GET /v1/videos/{代理商任务ID}/content
```

代理商内部请求：

```http
GET {BaseURL}/v1/videos/{Molii任务ID}/content
Authorization: Bearer {实例专属 Key}
```

代理必须支持：

- GET 与 HEAD；
- Range 与 If-Range；
- Content-Type、Content-Length、Content-Range；
- inline 与 attachment；
- 上游状态码的安全映射。

终端用户看不到 Molii Key、Molii 任务 ID、StarAI/COS 地址或临时签名。内容读取不依赖轮询响应中的短时 URL。

内容暂时不可用时返回明确的 502/503，但不修改任务状态、不重新生成、不重复结算。

## 8. 两层独立计费

Molii 使用自己的批发价格和代理商实例专属 Key 结算。代理商使用自己的价格、分组倍率、特殊倍率和余额向终端用户结算。模型同步不传递任何价格。

两层分别执行：

- 提交时预扣；
- 成功时按实际 Token差额结算；
- 失败时退款；
- 使用提交时价格快照；
- 使用持久化账单任务保证重启恢复和幂等。

代理商只消费上游返回的 `total_tokens` 或 `completion_tokens` 用量事实，再以本地价格计算，不复用 Molii 已计算金额。

任务成功但没有可靠用量时：不得显示零消费、不得猜测、不得静默将预扣视为最终金额；账单进入 `review_required` 等待检查或重试。

## 9. 状态、错误与重试

任务状态使用现有 `SUBMITTED → IN_PROGRESS → SUCCESS/FAILURE` 流程。

网络超时、连接中断、429、502、503 和 504保持任务原状态并重试。上游明确失败后才把本地任务标记失败并退款。认证错误记录为渠道配置故障，不向终端用户泄露 Key。

错误响应保留业务 `code`、`message`、`type`，但移除：

- API Key；
- 内部 URL；
- 数据库和堆栈信息；
- StarAI 私有诊断字段；
- 多层 JSON 字符串转义。

素材错误写入素材绑定；任务错误写入任务和使用日志。代理商管理员可看 Molii task ID，普通用户只看安全业务错误。

模型同步失败不清空渠道模型。服务重启后，数据库中的未完成任务和账单任务由现有系统任务重新接管。

## 10. 管理界面与可观测性

后台渠道类型显示 ByteDance 图标和 `ByteDance Seedance`。不显示 StarAI 专用价格表、StarAI Key、Molii 批发价、Molii 渠道 ID或 StarAI upstream task ID。

代理商任务详情显示：

- 本地 task ID；
- Molii task ID；
- 请求 ID；
- 本地提交耗时；
- Molii 排队和生成时间；
- 本地轮询发现延迟；
- 总耗时；
- 结算状态；
- 模型、分辨率、时长；
- 输入图片、视频和音频数量。

临时素材页面沿用现有创建、类型筛选、单项刷新、批量刷新和状态错误展示。文案改为中性的临时素材服务，不把代理商上游错误地称为 StarAI。

## 11. 兼容性与迁移

新增渠道不修改或迁移：

- 现有 Molii Volcengine Imagine API 渠道；
- 历史 StarAI 任务、日志和素材；
- 现有模型元数据和价格；
- 用户、Key、余额和分组；
- StarAI/COS 持久化与结算。

新渠道使用新的渠道类型编号和任务平台标识。历史数据继续按旧类型读取。代理商升级后创建新渠道、填写 Base URL与实例专属 Key、同步模型并配置本地零售价格即可。

## 12. 安全要求

- Base URL必须是绝对 HTTP(S) URL；生产建议 HTTPS；
- 禁止 Base URL直接指向当前实例自身；
- 上游请求只发送必要 Authorization 和内容协商头；
- 日志、错误、任务 Data 和 API 响应不能包含 Key；
- 内容代理复用 SSRF、重定向、响应大小、超时和 Header 白名单保护；
- 素材所有权先在本地校验，再访问上游；
- 模型授权以实例专属 Key 的 `/v1/models` 结果为准；
- 普通用户不能读取管理员诊断字段。

## 13. 验收标准

- 代理商仅凭 Base URL和实例专属 Key完成渠道创建；
- 自动获得且只能使用该 Key授权的 Seedance 模型；
- 图片、视频、音频素材可创建、查询、刷新、删除和用于生成；
- 用户和代理商实例之间的 asset ID互相隔离；
- 四个当前 Seedance 模型及各自媒体数量限制保持正确；
- 本地 task ID、Molii task ID和 StarAI task ID不会串层或越权展示；
- 提交、轮询、失败退款、成功结算和重启恢复通过；
- 视频预览、HEAD、Range 和下载通过，且不复制视频；
- 上游失败信息可读，不泄露凭据或私有诊断；
- 现有 StarAI 渠道全部回归测试通过；
- Go 测试、前端测试、类型检查、Lint 和生产构建通过。

## 14. 分析约束

项目 CCG 要求 L+ 任务进行 antigravity 与 Claude 双模型分析和审查。本机未安装约定的 `codeagent-wrapper`，因此本设计没有声称完成外部双模型分析。实施阶段如工具恢复，应补做双模型分析和最终审查；否则必须继续如实记录缺失，并通过针对性测试和人工审查降低风险。
