# 审查结果

## Critical

- 无。

## Warning

- 项目要求的 antigravity 与 Claude 双模型分析、审查均已并行尝试，但本机缺少 `~/.claude/bin/codeagent-wrapper`，四次调用均以状态码 127 退出，因此无法取得外部模型审查结果。

## Info

- `upstream_id` 仅从已脱敏的 `data.data.upstream_id` 读取，不回退到内部 `task_id` 或 `TaskPrivateData.UpstreamTaskID`。
- `/v1/videos/{task_id}` 与 `/v1/video/generations/{task_id}` 均将该字段放在各自响应数据的顶层；缺失时由 `omitempty` 省略。
- `id` 与 `task_id` 继续使用 Molii 公共任务 ID。
- 文档明确该字段仅用于技术支持排障，不能用于轮询或下载。
- 代码审查时发现创建响应示例误加了 `upstream_id`，已移除，确保只有查询响应声明该字段。

## 验证

- `go test ./... -count=1`
- `go test ./... -count=1`（`relaykit` 独立 module）
- `bun test scripts/seedance-content-contract.test.ts scripts/mdx-api-reference-contract.test.ts scripts/prepare-openapi.test.ts`
- curl 下载安全测试隔离重跑通过（组合测试首次超过 5 秒超时）。
- `bun run api:lint`
- `bun run build`
