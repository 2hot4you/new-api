# 实施计划

详细计划：`docs/superpowers/plans/2026-08-06-grok-video-final-usage-logs.md`

## 文件归属

- 后端：`model/task.go`、`controller/relay.go`、`service/task_billing.go`、`service/task_polling.go`、`service/grok_video_billing*.go`、`relay/channel/task/moliigrok/*` 及对应 Go 测试。
- 前端：`web/src/features/usage-logs/types.ts`、`lib/grok-video-billing*`、`components/dialogs/grok-video-billing-card.tsx`、日志详情/列表接入、i18n 和前端测试。
- 集成：根代理负责完整测试、构建、真实任务验证、安全审查、任务归档和最终本地提交。

## 执行顺序

1. 后端计费快照与终态单日志。
2. 前端结构化解析与计费卡片，可在后端契约确定后并行实现。
3. 集成完整回归、真实异步任务验证和任务归档。

用户明确禁止 antigravity 或 Claude；本任务不调用外部模型，也不 push。
