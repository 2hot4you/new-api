# Grok 图片使用日志与计费详情设计

## 目标

让 Molii Grok Imagine API 的图片生成、图片编辑消费日志同时可在“通用日志”和“绘图日志 > Image API”查看，并在详情中准确展示模型、分辨率、画面比例、输入/输出图片数量、真实单价、计费公式和最终扣费。

## 已确认的现状

- Grok 图片同步接口写入通用 `logs` 表；现有 `/usage-logs/drawing` 只查询 Midjourney 的 `/api/mj`，因此看不到 Grok 图片日志。
- Grok 图片用 `model_price = 1` 作为内部人民币直算锚点，前端误把它显示为真实“模型价格 ¥1”。
- 通用图片响应没有 token usage，后端为复用固定价结算补入 `prompt_tokens = 1`，前端因此显示了无业务意义的 Token 明细。
- Grok 的 `resolution`、`aspect_ratio`、输入图片数、实际输出数和分项单价只存在于请求或 Gin context，没有写入消费日志。

## 数据与接口设计

不新增日志表、不复制日志记录、不做数据库迁移。每次调用仍只写一条 `logs` 消费记录。

`GET /api/log` 与 `GET /api/log/self` 新增可选查询参数：

```text
log_category=grok_image
```

指定时，仅返回模型 ID 为 `grok-imagine-image` 或 `grok-imagine-image-quality` 的日志；不指定时保持现有行为，通用日志仍包含这些记录。筛选在数据库查询中完成，保证分页和 total 正确，并兼容 SQLite、MySQL、PostgreSQL 与 ClickHouse 日志库。

## 计费快照

在 `RelayInfo` 保存一次请求的类型化计费快照。请求估算阶段记录标准化参数与请求数量；上游成功后用实际返回图片数更新结算数据。消费日志的 `other` 新增：

```json
{
  "grok_image_billing": {
    "version": 1,
    "model": "grok-imagine-image-quality",
    "operation": "edit",
    "resolution": "2k",
    "aspect_ratio": "16:9",
    "requested_output_count": 2,
    "output_count": 1,
    "input_image_count": 2,
    "output_unit_price": 0.07,
    "input_unit_price": 0.01,
    "output_cost": 0.07,
    "input_cost": 0.02,
    "subtotal": 0.09,
    "group_ratio": 1,
    "final_cost": 0.09
  }
}
```

`content` 使用实际数据写入可读摘要，不再写误导性的 `quality=standard`：

```text
Grok 图片编辑, 模型 grok-imagine-image-quality, 分辨率 2K, 比例 16:9, 输出 1 张, 输入 2 张, 计费 (¥0.070000 × 1 + ¥0.010000 × 2) × 1.0000 = ¥0.090000
```

最终扣费仍以已有 `Log.Quota` 为账务真值；快照用于审计和展示。价格以请求发生时的配置快照为准，后续改价不会重写历史公式。

## 前端设计

`/usage-logs/drawing` 顶部新增二级标签：

- Image API：默认，查询 `/api/log?log_category=grok_image`，复用通用消费日志的筛选、表格与详情能力。
- Midjourney：保留现有 `/api/mj` 数据源、筛选和列。

两种数据源独立分页，不在浏览器端合并结果。

Grok 图片日志使用专用计费卡，展示：

- 模型 ID
- 操作（图片生成/图片编辑）
- 分辨率
- 画面比例
- 请求输出数与实际输出数
- 输入图片数（仅编辑时）
- 输出/输入单价及小计
- 分组倍率
- 完整计费公式
- 最终扣费

对于历史 Grok 图片日志，如果没有 `grok_image_billing.version = 1`，只展示模型 ID、最终扣费与“历史记录缺少分项计费数据”提示；不显示 ¥1、Token 明细或推测公式。

列表和移动端同样识别 Grok 图片日志：隐藏伪 `1 / 0` Token 与锚点价格，使用分辨率、比例和实际输出数作为计费摘要。

## 安全与兼容性

- 不信任上游返回的费用字段；所有费用继续使用 Molii 后台价格配置和实际输出数量计算。
- 上游实际输出数不得超过请求数；沿用现有响应校验。
- 新字段仅追加，不改变旧日志结构，不回填无法可靠恢复参数的历史数据。
- 分类仅匹配两个明确的 Grok 图片模型，不影响 Grok 视频、Seedance 或其他按次计费模型。

## 验收标准

- 同一条新 Grok 图片消费记录能在通用日志和绘图日志 Image API 中查到，数据库中只有一条记录。
- 生成和编辑日志均显示真实模型、分辨率、比例、数量、单价、公式和最终扣费。
- 实际返回数少于请求数时，以实际返回数计费并展示。
- Grok 图片日志不再显示模型价格 ¥1、Per-call 锚点或 Token 1/0。
- Midjourney、Seedance 和普通通用日志展示不回归。

