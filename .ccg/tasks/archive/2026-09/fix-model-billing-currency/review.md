# 审查结果

## 范围

- 模型级 `USD` / `CNY` 持久化、校验和一次性历史回填。
- `/api/pricing` 币种与安全分组元数据契约。
- 模型元数据编辑、模型广场价格展示、产品报价兼容。
- `/pricing` 桌面端和移动端分组筛选图标。

## 结论

- Critical：无。
- Warning：全仓前端 lint 被既有文件中的规则错误阻断；本次生产文件定向 lint 通过。
- Warning：完整前端套件受未跟踪的 `api-key-group-combobox.test.tsx`（错误引入 `node:test`）及既有 Grok 图片预览用例失败影响；本次相关 69 项回归通过。
- Warning：完整 `go test ./...` 在 controller 的共享全局数据库隔离问题下失败；独立运行 `go test ./model ./controller -count=1` 通过。
- Info：公开接口新增 `group_metadata`，不改变旧 `usable_group` 结构；字段仅包含当前可用分组的 icon 和 description。
- Info：历史人民币模型只回填一次；迁移标记完成后不会覆盖管理员后续选择。

## 审查限制

CCG 配置的 antigravity 与 Claude wrapper 均不存在，两路外部分析/审查无法运行（退出码 127）。受当前上级指令限制，本任务也未创建审查子代理。以上结论来自本地差异审查、定向静态检查与自动化测试，不声称完成外部双模型审查。

## 验证证据

- PostgreSQL 15 临时实例：模型币种回填与重复执行测试通过。
- Go：`go test ./model ./controller -count=1` 通过。
- Vitest：币种、价格编辑、分组筛选与报价相关 69 项通过。
- TypeScript：`pnpm typecheck` 通过。
- 生产构建：`pnpm build` 通过。
- 定向 lint：本次生产 TypeScript/TSX 文件通过。
