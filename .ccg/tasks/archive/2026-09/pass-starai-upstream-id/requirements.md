# 需求

- 解析 Molii Volcengine Imagine API 查询视频任务响应中的 `data.data.upstream_id`。
- `GET /v1/videos/{task_id}` 在响应顶层可选返回 `upstream_id`。
- `GET /v1/video/generations/{task_id}` 在响应数据顶层可选返回 `upstream_id`。
- `id` 与 `task_id` 继续使用 Molii 公共任务 ID。
- 不暴露内部渠道任务 ID、鉴权信息或签名参数。
- 上游未返回 `upstream_id` 时省略字段。
- 增加解析、序列化及脱敏边界的回归测试。

# 环境限制

- 已按项目要求并行尝试 antigravity 与 Claude 分析，但本机缺少 `~/.claude/bin/codeagent-wrapper`，两个调用均以 127 退出。
