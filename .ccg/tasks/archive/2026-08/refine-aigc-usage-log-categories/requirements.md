# 需求

- 将用户可见的“任务日志”统一调整为“生成记录”。
- 将一级分类“绘图日志 / 任务日志”调整为“图像生成 / 视频生成”。
- 图像生成当前只展示 `Grok Image` 分类，保留后续扩展 Seedream、GPT Image 等模型族的结构。
- 视频生成当前展示 `Grok Video` 与 `Seedance` 分类，保留后续扩展其他视频模型族的结构。
- 删除日志页面的 Midjourney 分类入口。
- 分类必须真实筛选数据：Grok Image 使用专属日志类别，Grok Video 与 Seedance 使用各自任务平台。
- 保留现有 `/usage-logs/drawing` 与 `/usage-logs/task` URL，避免破坏已有链接和权限配置。
- 不删除后端 Midjourney API 或数据结构，减少与上游代码的冲突；只移除 Molii 日志页面入口。

