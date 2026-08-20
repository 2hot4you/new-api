# 手动模型广场排序审查

## 结论

- 最终结论：通过，可合并。
- Critical：0。
- Important：0。
- Minor：1（非阻塞）：管理员模型排序 GET 当前返回完整 `Model` 记录，而不是轻量排序 DTO；功能与安全正确，但在接近 10,000 条上限时有额外序列化成本。

## 已验证范围

- `models` 与 `vendors` 独立持久化 `display_order`。
- 新增、删除、初始化、目录对账和重排共享数据库单例锁；完整 ID 集合校验与更新位于同一事务。
- PostgreSQL 显式迁移可重复执行，并覆盖并发新增与陈旧重排冲突；SQLite 覆盖并发与回滚。MySQL 复用标准行锁语义，尚无独立集成环境。
- 管理端 GET/PUT 排序接口位于 AdminAuth 下，静态 `/order` 路由早于 `/:id`，成功后刷新定价缓存。
- `/pricing` 默认保留后端手动顺序；移除“最新发布”，保留推荐、名称与价格排序；历史 `sort=release-date` 和无效值归一为推荐。
- 模型与供应商排序界面读取完整列表，支持拖拽、触屏、键盘上下移动、保存/取消、失败保留草稿、保存期交互锁定和缓存刷新。
- 模型草稿不会被焦点/重连 refetch 覆盖；供应商加载可取消/关闭，迟到响应被忽略并支持重试。
- 七种 Web 语言包含全部排序文案及四个稳定后端错误消息；订单 API 禁用全局 business-error toast，避免同时显示未翻译错误。

## 验证结果

- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `go build ./...`：通过。
- `sh deploy/migrations/20260820_marketplace_display_order_test.sh`：通过（PostgreSQL 15，双次迁移与并发契约）。
- 相关前端测试：60/60 通过。
- `bun run typecheck`：通过。
- `bun run i18n:check`：通过。
- `bun run build`：通过。
- 变更文件 scoped oxlint、格式检查与 `git diff --check`：通过。

## 多模型审查

按仓库要求并行尝试 antigravity 与 Claude 审查；本机缺少 `~/.claude/bin/codeagent-wrapper`，两次调用均以 127 退出。已由多个独立 CCG review Agent 分层审查数据库/API/定价/UI/i18n，并在修复所有 Critical/Important 后完成最终跨功能复审。

## Spec 回馈

本次没有需要新增到 `.ccg/spec` 的通用规范；并发排序采用项目内持久单例锁与完整集合校验，已由代码和测试直接固化。
