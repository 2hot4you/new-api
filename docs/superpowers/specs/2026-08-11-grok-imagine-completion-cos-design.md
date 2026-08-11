# Molii Grok Imagine API 能力补齐与 COS 持久化设计

**状态：** 已确认  
**日期：** 2026-08-11  
**范围：** 用户侧 Grok Imagine 图片、视频和 Files API

## 1. 目标与边界

本次按 P0 → P1 → P2 补齐 Molii Grok Imagine API，并把图片、视频结果和用户文件统一保存到现有腾讯云 COS。对象在 Molii 中只保留 24 小时。

本次明确不做：

- 不支持 `grok-imagine-video-1.5-preview`；公开目录、参数和接口只保留正式模型 ID。
- 不设计渠道切换、渠道亲和或模型映射；Molii Grok Imagine API 和 Molii Volcengine Imagine API 均按单渠道使用。
- 不直接暴露或透传上游 Files ID。
- 不让上游 `cost_in_usd_ticks` 决定 Molii 用户计费；它只可作为管理员审计元数据。
- 不自动修改 COS Bucket 生命周期规则，不新增 Docker 或云部署流程。
- 不新增 `reference_audios`、Voice ID、Share、附件聊天等未请求能力。

## 2. 方案比较与选择

### 方案 A：Molii 自有 COS 与本地 Files API（采用）

Molii 下载上游结果并存入现有 COS，以本地文件记录和对象键作为唯一持久化依据。用户的 `file_id` 只在 Molii 内有效，转给上游前解析为短期签名 URL。

优点是用户隔离明确、结果链接稳定、24 小时生命周期可控，也不会把上游账号和文件能力泄漏给用户。代价是需要实现媒体探测、复制重试、文件元数据和清理机制。

### 方案 B：直接使用上游 Files API

实现较快，但文件与渠道账号强绑定，容易产生跨用户读取、渠道变化失效和上游临时链接泄漏，不采用。

### 方案 C：只代理上游临时链接

改动最小，但无法保证 24 小时可用性，已出现过链接过期和播放失败问题，不采用。

## 3. P0：协议别名与视频编辑精确计费

### 3.1 官方视频生成路径

新增 `POST /v1/videos/generations`，与现有 `POST /v1/videos` 共用同一个提交处理器、鉴权、校验、预扣、异步轮询和日志链路。旧路径继续保留，避免破坏已有客户。

### 3.2 安全媒体探测

新增独立媒体探测服务，用于读取 MP4 的时长、宽度和高度：

- 优先读取 Molii 本地 Files 元数据；缺失时再探测用户 URL 或 Data URL。
- 远程 URL 必须使用现有 SSRF 防护客户端，禁止私网、非法端口、凭据 URL和不安全重定向。
- 只接受 MP4 与受支持编码，限制响应体大小和超时；使用已有 `github.com/abema/go-mp4`，不依赖外部 `ffprobe`。
- 探测结果统一为 `duration_seconds`、`width`、`height`、`resolution_tier`，其中计费档仅允许 `480p` 或 `720p`。
- 探测失败发生在付费提交前，返回稳定的 400 错误，不向上游发起生成请求，也不预扣。

### 3.3 视频编辑结算

`grok-imagine-video` 编辑输出继承输入视频时长和宽高，分辨率最高按 720p 计算。提交时把探测结果写入不可变计费快照，来源标记为 `input_probe_v1`；轮询响应若提供合法分辨率，可用于一致性检查，但不得依赖上游费用反推档位。

最终公式：

```text
总价 = 输入视频秒数 × 输入视频单价
     + 输出视频秒数 × 对应分辨率输出单价
     + 工具调用费用
```

如果完成阶段发现输入快照缺失或不一致，任务结果仍可保存，但账务作业进入 `review_required`，保留预扣并等待人工处理，不猜价格。

## 4. P1：视频扩展、参考图视频与结果持久化

### 4.1 视频扩展

新增 `POST /v1/videos/extensions`：

- 仅支持 `grok-imagine-video`。
- `prompt` 和 `video` 必填。
- 输入必须为 MP4，输入时长 2–15 秒。
- `duration` 为新增时长，允许 2–10 秒，默认 6 秒。
- 不接受 `aspect_ratio` 和 `resolution`。
- 输出继承输入画幅与分辨率，最高 720p。

提交前完成媒体探测并写入输入秒数、输出秒数和分辨率快照。创建请求不可自动重放；查询和结果复制可幂等重试。

### 4.2 Reference-to-Video

在视频生成请求增加 `reference_images`：

- 支持 `grok-imagine-video-1.5` 正式模型。
- 允许 1–7 张参考图，每张只接受 URL、Data URL 或 Molii `file_id`。
- 参考图模式最高 720p。
- `reference_images` 与单图 `image`、视频编辑、视频扩展互斥。
- 请求至少包含参考图；本轮不开放参考音频。

### 4.3 COS 目录与 24 小时生命周期

复用 Seedance 已有 COS 的 SecretId、SecretKey、Region、Bucket、公共域名和 PathPrefix，仅新增 Grok 独立目录：

```text
{PathPrefix}/grok-results/{userID}/image/YYYY/MM/{random}.{ext}
{PathPrefix}/grok-results/{userID}/video/YYYY/MM/{random}.mp4
{PathPrefix}/grok-files/{userID}/YYYY/MM/{random}.{ext}
```

应用层为每个对象保存 `object_key`、`expires_at` 和媒体元数据，固定 `expires_at = created_at + 24h`。访问代理在到期后立即返回 410；后台清理器通过独立 Redis 有序集合删除到期对象。管理员另在腾讯云控制台为 `grok-results/` 和 `grok-files/` 前缀配置 1 天删除规则作为兜底。

代码只保存对象键，不保存长期预签名 URL；每次下载或预览时生成短期签名 URL。

### 4.4 图片结果持久化

图片生成和编辑成功后，先把所有返回图片复制到 COS，再返回 Molii 结果地址并完成最终扣费：

- 多图采用全成全败；任何一张复制失败时删除本轮已复制对象。
- 复制失败返回可重试的服务错误，并按现有同步请求账务规则退款。
- 保留 `mime_type` 和 `revised_prompt` 等用户可见字段；上游费用只进入管理员审计字段。
- 日志只保存 Molii 结果标识，不保存上游签名 URL。

### 4.5 视频结果持久化

异步轮询到上游成功后，先把视频复制到 COS，再提交 SUCCESS 终态与结算作业：

- COS 复制失败时任务保持可重试的处理中状态，不提前对用户显示成功。
- 任务私有数据保存对象键、过期时间、媒体类型和探测到的实际元数据。
- `GET /v1/videos/{task_id}/content` 与平台预览始终通过 Molii 代理读取 COS，并支持 Range 请求。
- 重复轮询和重复复制以任务 ID + 结果类型作为幂等键，不重复创建对象或扣费。

## 5. P2：Molii 本地 Files API

替换现有 `/v1/files` 未实现处理器，提供用户级接口：

```text
POST   /v1/files
GET    /v1/files
GET    /v1/files/{file_id}
GET    /v1/files/{file_id}/content
DELETE /v1/files/{file_id}
```

文件记录至少包含：公共 `file_id`、`user_id`、文件名、purpose、MIME、大小、COS 对象键、宽高、时长、状态、创建时间和过期时间。

安全约束：

- 所有读取、下载和删除必须同时校验当前 UserID，不能只依赖不可猜测 ID。
- 文件固定 24 小时过期；到期后 API 返回 410，清理器异步删除 COS 对象与安全元数据。
- Grok 适配器接收 `file_id` 后只解析当前用户文件，生成短期 COS URL传给上游。
- 图片生成/编辑、视频生成/编辑/扩展、参考图都使用同一解析器。
- 上传格式、大小和媒体元数据在写入完成前验证；失败对象立即清理。

## 6. 数据与模块边界

新增或泛化以下职责，不把逻辑继续堆入适配器：

- `object storage`：provider-neutral COS 上传、复制、签名、删除和对象键生成；保留原 Seedance 包装器以避免回归。
- `media probe`：安全下载/解码媒体元数据，只返回规范化结果。
- `Grok result persistence`：图片和视频的幂等复制、24 小时过期和清理队列。
- `Files`：文件记录、UserID ownership、上传/列举/下载/删除。
- `Grok adaptor`：只负责协议校验、上游请求转换和结果解析。
- `billing snapshot`：保存请求模型、动作、输入媒体维度、输出维度和所用价格版本。

数据库新增文件表和必要索引；任务结果对象信息继续保存于任务私有 JSON，避免为 Grok 单独扩展任务表列。提供 PostgreSQL 幂等迁移文件，不提交真实 COS 密钥。

## 7. 错误与账务原则

- 用户输入、格式、媒体探测和 ownership 错误：400/404/410，在预扣与上游请求前结束。
- COS 临时故障：同步图片返回可重试 5xx；异步视频保持处理中并按退避重试。
- 上游终态失败：进入现有退款账务作业。
- 上游成功但计费维度无法验证：保留结果，账务进入 `review_required`。
- 资金与任务终态继续使用现有 billing outbox，所有操作必须幂等。
- 公开错误和日志不得出现上游渠道名、密钥、签名 URL 或原始响应。

## 8. 测试与验收

全部功能按 RED → GREEN → REFACTOR 实施，不发送真实付费请求。至少覆盖：

- 新旧 generation 路由行为一致。
- MP4 探测、SSRF、重定向、体积、超时、非法 MIME 和 Data URL。
- edit/extension 的时长、分辨率、互斥参数和精确计费快照。
- reference_images 数量、模型、互斥和 file ownership。
- COS 对象键前缀、24 小时过期、幂等复制、部分失败回滚和到期拒绝。
- 图片多结果全成全败；视频在 COS 完成前不进入 SUCCESS。
- Files API 的同用户成功、跨用户 404、过期 410、删除幂等。
- billing outbox 的成功、失败、重试和 `review_required`。
- 定向 Go 测试、根模块 `go test ./...`、`go vet ./...`、`gofmt` 与 `git diff --check`。

本地开发只运行 Go 后端与现有前端，不制作 Docker 镜像，不执行生产部署。
