# 审查记录

## 结果

- 后端规格与代码质量复审：通过，无 Critical/Warning。
- 前端规格与代码质量复审：通过，无 Critical/Warning。
- 用户明确禁止外部模型，本任务未调用 antigravity 或 Claude。

## 已修复问题

- 退款失败时不再写终态错误日志，普通轮询与超时清理均受退款成功结果约束。
- Grok V1 计费快照未完成实际结果计算时，不再把预扣金额记录成最终消费。
- 仅在提交快照完整有效时启用 `final_usage_log_only`，无效快照回退旧日志路径。
- V1 最终额度统一使用 half-away-from-zero 取整。
- Grok 视频失败日志在前端保留脱敏错误内容，不进入历史计费卡片。
- 去除新增的重复 i18n 键，并补充详情弹窗与列表路由测试。

## 验证证据

- `go test ./...`：通过。
- `go build`：通过。
- `go vet ./service ./model ./controller ./relay/channel/task/moliigrok`：通过。
- `bun test src/features/usage-logs`：28/28 通过。
- `pnpm typecheck`：通过。
- `pnpm build`：通过。
- `git diff --check`：通过。
- 变更文件安全扫描：未发现真实 API 密钥；命中项均为测试占位符或 fixture URL。
- 本地真实任务 `task_beBvXTpd3eeeyMAXcNEGdQkL5AE8HrKg`：提交阶段 0 条日志；成功后仅 1 条 type=2 消费日志、0 条退款日志，最终 quota=25000（¥0.05）。
- 实测 V1 快照：`text_to_video`、1 秒、480p，公式 `(¥0.050000 × 1) × 1.0000 = ¥0.050000`；日志未包含媒体 URL、上游 ID 或密钥。
- LaunchAgent 重建并重启后 `/api/status` 返回 200。

## 已知非阻塞项

- 控制器提交分支没有完整的端到端测试；当前由快照门控单测、终态统计测试和本地接口验证共同覆盖。
- 令牌额度更新仍沿用项目现有的 best-effort 语义，资金调整与任务 quota 标记回写也不是全局事务。本次未扩大为跨模块账本重构。
