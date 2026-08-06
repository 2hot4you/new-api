# 生成日志清理与 Grok 模型广场完善设计

## 背景与根因

后台日志维护的异步清理任务目前只调用 `CountOldLog` 和 `DeleteOldLogBatch`，因此只处理 `logs` 表。生成记录中的 Grok、Seedance 视频任务来自独立的 `tasks` 表，所以清理任务虽然成功，历史视频记录仍然存在。

模型广场当前按端点类型复用通用详情组件。Grok 图片因此继承了文本模型的 TPS、TTFT 和 Token/s 性能文案，并生成包含 `size` 的通用 OpenAI 图片样例；Grok 视频则继承 Seedance 的 `content`、`generate_audio`、帧率和多模态素材说明。上述内容均与 Molii Grok Imagine API 的实际请求契约不一致。此外，视频性能查询固定筛选渠道 61，导致渠道 62 的 Grok 视频任务无法进入性能统计。

## 方案与边界

采用“现有框架内的 Grok 专用模块”方案。通用模型详情、Seedance 详情和现有计费逻辑保持不变；仅在识别到四个 Molii Grok 模型时切换到专用概览、性能、价格和 API 数据。

支持的模型固定为：

- `grok-imagine-image`
- `grok-imagine-image-quality`
- `grok-imagine-video`
- `grok-imagine-video-1.5`

本次不抽象 Seedream、GPT Image 等尚未接入的模型，也不重新设计模型广场整体布局。

## 历史日志与生成任务清理

### 数据范围

一次清理任务使用同一个 `target_timestamp`：

- 删除 `logs.created_at < target_timestamp` 的普通日志。
- 删除 `tasks.created_at < target_timestamp` 且 `status IN (SUCCESS, FAILURE)` 的生成任务。
- 保留所有非终态任务，不因任务提交时间较早而删除。
- 不删除 `system_tasks`、用户、渠道、Token、资产或计费配置。

### 执行与进度

沿用现有异步系统任务和分批删除机制，不增加新的管理入口。清理开始时分别统计普通日志和可清理生成任务，合计形成总进度；执行时先分批清理普通日志，再分批清理终态生成任务。每次删除仍带时间和状态条件，避免误删活动任务。

状态与结果保持向后兼容：

- 现有 `total`、`processed`、`remaining`、`progress` 和 `deleted_count` 继续表示两类记录的合计。
- 新增 `deleted_log_count` 和 `deleted_generation_count`，供前端显示分类结果。
- 前端完成提示明确显示普通日志和生成记录的删除数量；无匹配项时保留现有空结果提示。

数据库删除使用 GORM 条件查询，兼容 SQLite、MySQL 和 PostgreSQL。`tasks` 位于主数据库，不走 ClickHouse 日志库；ClickHouse 的普通日志删除继续使用现有同步 mutation 路径。

## Grok 模型概览与价格

### 模型简介

后端默认模型描述注册表为四个模型提供稳定的 i18n key，前端至少补齐中文和英文翻译。数据库中已有管理员自定义描述时，继续优先显示自定义内容。

### 图片模型概览

图片模型显示与实际适配器一致的信息：

- 文本生成图片和图片编辑。
- `1k`、`2k` 输出分辨率。
- `n` 为 1–4。
- 编辑请求接受 1–3 张输入图片。
- 支持后端 `allowedAspectRatios` 中的画幅比例，默认 `16:9`。

概览价格不显示仅用于内部扣费换算的 `¥1` 模型锚点。后端定价响应增加 Grok 图片直接价格结构，包含当前配置的输入图片单价以及 1K、2K 输出单价；前端以图片计费矩阵展示这些真实价格。

### 视频模型概览

`grok-imagine-video` 展示文生视频、图生视频和视频编辑能力，生成支持 480p/720p、1–15 秒，编辑入口不展示生成专用的时长、画幅和分辨率参数。

`grok-imagine-video-1.5` 展示必需图片输入的图生视频能力，支持 480p/720p/1080p、1–15 秒，不宣称支持视频编辑。

两个 Grok 视频模型都不展示帧率、Seedance 素材 ID、`content` 数组、`generate_audio`、Web Search 或生成音频能力。

## Grok 性能

### 图片性能

继续复用现有 `perf_metrics` 聚合数据，不新增表。后端把已经存在但未序列化的分组 `request_count` 和时间桶 `request_count` 暴露给前端。

Grok 图片专用性能组件展示：

- 最近 24 小时请求量。
- 平均完整响应时间。
- 成功率。
- 按分组的请求量、平均响应时间和成功率。
- 响应时间趋势和成功率趋势。

组件中不出现 TPS、TTFT、Token/s、“每秒生成 Token”或任何 Token 计费暗示。普通文本模型继续使用原性能组件。

### 视频性能

将视频性能查询从固定渠道 61 改为按模型选择任务平台：Seedance 模型查询渠道 61，四个 Grok 模型中的视频模型查询渠道 62。聚合结构继续使用已提交、成功、失败、处理中、平均/P50/P95 生成耗时和分组统计，不引入 Token 指标。

未知视频模型保持安全空结果，不跨平台混合任务。

## Grok API 页面

### 识别与结构

新增 Grok 模型识别和专用样例构建模块。模型详情主组件仅负责选择通用或 Grok 内容，避免把模型特例继续堆入大组件。

API 页面继续保留语言标签，支持 cURL、Python、TypeScript 和 JavaScript，并使用状态接口提供的实际 Base URL；示例密钥只使用环境变量或 `<YOUR_API_KEY>` 占位符。

### 图片模型

图片 API 覆盖：

- `POST /v1/images/generations`
- `POST /v1/images/edits`

参数表基于真实校验：`model`、`prompt`、`aspect_ratio`、`resolution`、`n`，编辑额外包含 `image` 或 `images`。样例使用 `resolution` 和 `aspect_ratio`，不再使用后端未采纳的通用 `size`、`quality`、`style` 或 `response_format`。

### 视频模型

视频 API 覆盖：

- `POST /v1/videos`
- `POST /v1/videos/edits`，仅 `grok-imagine-video`
- `GET /v1/videos/{task_id}`
- `GET /v1/videos/{task_id}/content`

生成参数为 `model`、`prompt`、`image`、`duration`、`aspect_ratio`、`resolution`；`grok-imagine-video-1.5` 明确标记图片为必需。编辑参数仅为 `model`、`prompt` 和 `video`。示例展示创建任务、读取公开任务 ID、轮询状态和下载成功结果，不使用 Seedance 的 `content` 结构。

### 认证和错误提示

认证继续使用 `Authorization: Bearer <TOKEN>`。API 页说明异步视频任务应轮询到 `completed` 或 `failed`，只有成功任务才下载内容。示例对非 2xx 响应执行显式错误检查，不持久化测试凭据。

## 测试与验证

### 后端

- 先写失败测试，证明现有清理任务不会删除终态 `tasks`。
- 覆盖普通日志与终态任务合计、分类计数、批量删除和活动任务保留。
- 覆盖 Grok 图片直接价格进入 `/api/pricing` 且不以锚点作为展示价。
- 覆盖性能响应中的请求数序列化。
- 覆盖 Seedance 查询渠道 61、Grok 查询渠道 62以及未知模型空结果。
- 运行 `go test ./...` 和 `gofmt` 检查。

### 前端

- 先写失败测试覆盖四个 Grok 模型的概览、价格、性能字段和 API 样例。
- 断言 Grok 页面没有 TPS、TTFT、Token/s、Seedance `content` 或帧率文案。
- 断言图片生成/编辑、视频生成/编辑、查询和下载样例使用真实路由与参数。
- 运行 usage/pricing/system-settings 相关测试、TypeScript 类型检查、格式检查、lint 和生产构建。

### 浏览器

在 `http://127.0.0.1:3000` 验证日志维护确认框与完成提示，并分别打开四个 Grok 模型详情，检查概览、性能和 API 标签的内容、代码复制区域和控制台错误。

## 非目标

- 不清理仍在执行的生成任务。
- 不修改 New API 之外的独立 Molii 前端。
- 不改变 Seedance API 契约或详情展示。
- 不新增数据库迁移；本次仅使用现有表和新增 JSON 响应字段。
- 不提交真实密钥，不 push、不合并、不创建 PR。
