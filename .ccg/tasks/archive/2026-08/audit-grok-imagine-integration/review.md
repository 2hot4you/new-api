# Molii Grok Imagine API 对接缺口审计

审计日期：2026-08-11

## 已完成

- 图片生成与图片编辑：两个模型、1–3 张编辑输入、1K/2K、比例、数量、内容审核错误和直接人民币计费。
- 视频生成：`grok-imagine-video` 文生/图生，`grok-imagine-video-1.5` 图生，异步轮询、任务记录、结果代理和终态结算。
- 视频编辑：`grok-imagine-video`，异步轮询、任务记录和按输入/输出时长计费。
- 管理能力：New API 管理接口余额查询、`/v1/models` 模型获取、渠道可达性测试。
- `grok-imagine-video-1.5-preview` 已退出公开模型目录；不考虑渠道切换或模型映射。

## 建议补齐

### P0

1. 明确视频编辑最终分辨率来源。当前终态结算要求轮询响应提供 `video.resolution`；xAI 官方示例未承诺该字段，缺失时任务会进入 `review_required`，保留预扣。
2. 若对外承诺 xAI REST 路径兼容，增加 `POST /v1/videos/generations`；目前用户侧只有 `POST /v1/videos`。

### P1

3. 增加 `POST /v1/videos/extensions` 视频延长及独立计费快照。
4. 增加 `reference_images` 参考图生视频；它与首帧 `image` 不同，只适用于 `grok-imagine-video`。
5. 增加可选的持久化输出能力。当前图片返回上游临时 URL，视频结果代理仍依赖上游临时 URL；官方 `storage_options`/Files 输出可提供持久化资产。

### P2

6. 实现用户级 Files API 后再支持 `file_id`；当前明确返回 `400 file_id_not_supported`，这是安全的刻意限制。
7. 保留并展示上游响应元数据：图片 `mime_type`、`revised_prompt`、`usage.cost_in_usd_ticks`；视频 `respect_moderation`、`file_output`。其中 provider cost 可用于成本审计，但不直接替代 Molii 配置价。
8. 如需完整 xAI 模型发现，再补 `/v1/image-generation-models` 与 `/v1/video-generation-models`；当前管理侧 `/v1/models` 已满足现阶段余额/模型同步需求。

## 验证

`go test ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./service ./controller -run 'MoliiGrok|Grok(Image|Video)|TaskBilling|VideoProxy' -count=1` 通过。
