# Seedance 任务可观测性审查

## 结论

- Critical：无。
- Warning：无本次变更新增问题。
- Info：历史任务可能缺少标准化时间戳或素材数量，界面按设计显示“未记录”；新任务会写入完整快照。

## 正确性与权限

- 平台公共 `task_id` 从使用日志既有公开 `other.task_id` 读取，不暴露上游任务 ID。
- `upstream_task_id` 仍只存在于 Root-only 投影中，本次未改变其权限。
- 详细时间线仅写入 `TaskAdminInfo`，普通用户 DTO 测试确认不包含上游时间字段。
- 视频预览只公开图片、视频、音频数量，不保存或返回 Prompt、素材 URL、Asset ID 或原始 StarAI 响应。
- 缺失值使用可空字段，真实的 `0` 与历史“未记录”保持区分。
- 负耗时和上游/平台时钟倒退不参与计算，避免展示伪造数据。
- 请求 ID 跳转使用已有 `requestId` 查询参数，并将查询时间限制在任务起止时间前后各 5 分钟。

## 验证证据

- `go test ./...`：通过。
- `go vet ./...`：通过。
- 新增前端定向测试：4 个文件、8 个测试通过。
- `bun run typecheck`：通过。
- `bun run i18n:check`：通过。
- `bun run build`：通过。
- 受影响前端文件 `oxlint`：通过。
- 受影响 Go 文件 `gofmt -l`：无输出。
- `git diff --check`：通过。
- 新增前端文件版权头检查：通过。
- 全量前端测试：215 个文件、1501 个测试通过；唯一失败为既有 `src/lib/__tests__/site-brand.test.ts`，其使用 `node:test`，当前 Vitest 配置无法打包 Node 内置模块。本次未修改该文件或测试配置。

## 审查限制

项目要求的 antigravity + Claude 外部双模型包装器 `~/.claude/bin/codeagent-wrapper` 在当前机器不存在，因此无法执行外部双模型分析/审查。本次使用人工差异审查、完整 Go 测试、前端定向测试、类型检查、lint、国际化检查和生产构建替代；未声称完成双模型审查。

## Spec 回馈

当前工作树不存在 `.ccg/spec/backend/index.md`、`.ccg/spec/frontend/index.md` 或 `.ccg/spec/guides/index.md` 的可维护内容，因此未新增项目级规范。
