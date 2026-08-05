# 需求

- 在 Molii Grok Imagine API 渠道加入 `grok-imagine-video`，保留现有 `grok-imagine-video-1.5`。
- 支持 `grok-imagine-video` 的文生视频、图生视频和视频编辑调用。
- 按 1 美元数值等于 1 人民币元配置官方目录价，不执行汇率换算。
- 区分输出分辨率、输出时长和媒体输入计费，不把不同价格合并为单一模型价。
- `grok-imagine-video` 视频输入按 ¥0.01/秒、图片输入按 ¥0.002/张计费；输出 480p 按 ¥0.05/秒、720p 按 ¥0.07/秒计费。
- `grok-imagine-video-1.5` 图片输入按 ¥0.01/张；输出 480p/720p/1080p 分别按 ¥0.08/¥0.14/¥0.25 每秒计费。
- Grok 两个图像模型分别支持标准/高质量和 1K/2K 输出价格。
- 在现有 `molii-aigc-video-pricing` 设置入口中增加 Grok 图片、视频和工具调用价格表单。
- 按实际完成的 xAI 工具调用计费，支持 web、X 搜索、代码执行、附件搜索、集合搜索及图像生成。
- 提供单元测试、Go 格式检查、Go 测试和前端构建验证。
- 不调用 antigravity 或 Claude，不推送远端。
