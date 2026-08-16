# 本地模型广场元数据最终审核

## 结论

通过。`/models/metadata` 已成为模型广场资料的唯一可编辑来源；公开 `/api/pricing` 与 `/pricing` 只读取已持久化、显式发布且运行时可售的模型资料。未发现阻断交付的 Critical 或 Warning。

## 审核中发现并修复

1. 管理页面仍残留已失效的“官方同步”筛选、表格列和编辑开关。现已从管理 UI、表单映射和查询参数中移除；数据库旧列仅保留兼容性，不再参与页面逻辑。
2. PostgreSQL 迁移契约脚本可能撞上官方镜像初始化期间的临时 PostgreSQL 关闭窗口。现要求连续三次健康检查，并连续执行两轮契约测试通过。
3. 补充移除旧 `MODEL_METADATA_*` 环境变量示例，并修正价格页 API 分类函数的格式与过期注释。

## 验证证据

- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `bun test src/features/models src/features/pricing src/features/home`：118 项通过。
- `bun run typecheck`：通过。
- `bun run i18n:check`：通过。
- `bun run format:check`：通过。
- 变更文件定向 `oxlint`：通过。
- `sh deploy/migrations/20260815_model_marketplace_metadata_test.sh`：连续两轮通过，覆盖重复迁移、全新库收敛与并发管理员更新保护。
- 本地开发接口 `/`、`/api/status`、`/api/pricing`：HTTP 200。
- 本地 `/pricing`：展示已发布模型，未出现 models.dev 来源信息。
- 已登录管理页面 `/models/metadata`：列表加载成功；移除外部同步控制前后的页面契约由自动化测试覆盖。

## 已知基线

- 仓库完整 `bun run lint` 仍包含本任务之外的既有 lint 告警；本次变更文件定向检查、类型检查和全仓格式检查均通过。
- 本任务按用户明确要求未调用 antigravity 或 Claude；由主代理完成代码审查与运行时验收。
- 未运行生产 Web build，未访问外部模型元数据服务，未推送远端。
