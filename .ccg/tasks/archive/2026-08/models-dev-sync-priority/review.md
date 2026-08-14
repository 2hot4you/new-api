# Review

## Scope

- 手动 models.dev 同步新增 `local_first` 与 `models_dev_first` 两种优先级。
- 未提供同步模式时保持本地优先，自动同步始终保持本地优先。
- models.dev 优先仅覆盖公开模型元数据，不修改价格、渠道、启用状态、路由、分组或计费配置。
- 管理端同步弹窗提供明确选择、覆盖警告和多语言说明。

## Verification

- `go test ./... -count=1`：通过。
- `go vet ./model ./controller`：通过。
- `bun run typecheck`：通过。
- `bun run i18n:check`：通过。
- `bun test src/features/models/lib/__tests__/model-metadata-sync-mode.test.ts`：2/2 通过。
- 本次修改文件的 `oxlint`：通过。
- `bun run format:check`：通过。
- `git diff --check`：通过。
- 3000 端口后端重启后 `/api/status` 健康检查：通过。

## Findings

- Critical：0。
- Warning：0（本次变更范围内）。
- 已知基线：直接运行仓库全量 `bun test` 会因既有 `node:test` 与 Bun 测试运行器混用及既有用量日志用例失败而退出非零；本次相关测试、类型、翻译、格式和 lint 均通过，未扩大范围修复这些历史问题。

## External review

未调用 antigravity 或 Claude，遵守用户明确要求；本次采用本地测试与人工差异审查。
