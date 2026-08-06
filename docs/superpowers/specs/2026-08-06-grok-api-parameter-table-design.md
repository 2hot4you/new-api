# Grok API 参数表设计

## 目标

将模型广场 Grok 模型详情 API 标签中的参数摘要改为结构化参数表，使用户可以直接查看每个参数的类型、必填状态、默认值、可选值和用途。

## 展示方案

复用 `ModelDetailsApi` 已有的 `StaticDataTable`、表头与单元格样式，保持 Grok 参数表与其他模型详情一致。表格列为：

1. 参数：展示参数路径，并在必填参数旁显示“必填”徽标。
2. 类型：使用类型徽标展示 `string`、`integer`、`object` 等类型。
3. 默认值 / 可选值：默认值单独标识；枚举值使用代码标签展示；无默认值时不伪造数据。
4. 说明：使用简洁中文说明参数用途和操作限制。

## 参数数据

新增独立的 Grok 参数元数据构建函数，以模型 ID 和当前操作为输入。组件只负责渲染，不在 JSX 中拼装业务规则。

- 图片生成：`model`、`prompt`、`aspect_ratio`、`resolution`、`n`。
- 图片编辑：在生成参数基础上增加 `image` / `images` 输入说明。
- `grok-imagine-video` 生成：`model`、`prompt`、`image`、`duration`、`aspect_ratio`、`resolution`。
- `grok-imagine-video` 编辑：`model`、`prompt`、`video`。
- `grok-imagine-video-1.5` 生成：图片输入为必填，分辨率增加 `1080p`，不提供编辑操作。
- 任务状态与下载：仅展示路径参数 `task_id`。

参数范围以当前 Molii Grok 适配器已验证的请求格式为准，不展示上游未支持的字段。

## 组件边界

- 参数元数据放在 Grok 专用的纯 TypeScript 模块中，便于测试和后续扩展。
- 参数表渲染放在 API 详情组件中，并复用现有 `ParamRangeCell`。
- 操作标签变化后直接根据当前操作派生参数数组，不引入额外副作用或冗余状态。
- 非 Grok 模型继续使用现有 `buildSupportedParameters`，行为不变。

## 测试

- 先为每种模型/操作编写失败的数据层测试。
- 验证视频 1.5 的图片必填与 1080p，以及不支持编辑。
- 验证状态和下载只包含 `task_id`。
- 运行相关 Bun 测试、TypeScript 类型检查、格式检查和生产构建。
- 重启本地常驻后端后，在 `127.0.0.1:3000/pricing` 验证标签切换和表格渲染。

## 范围限制

- 不修改 API 契约或后端请求处理。
- 不重构通用模型详情页面。
- 不调用 antigravity 或 Claude。
- 不 push、合并或创建 PR。
