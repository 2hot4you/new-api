# 阶段审查记录

## v1.0.0-rc.28

- 保留 Molii 的 `SESSION_COOKIE_TRUSTED_URL` 安全语义，并加入上游可选密码传输加密配置。
- 接收上游 PostgreSQL 连接池兼容、SQLite WAL/busy timeout、JSON Valuer、响应头等待上限及任务插件别名解析修复。
- 保留 Molii “模型必须具备完整且已发布元数据才进入公开定价”的约束；为上游别名定价测试补齐公开模型夹具。
- 修复 controller 测试清理在恢复空数据库句柄后刷新全局渠道缓存导致的 panic；先用有效测试库清空并重建别名视图，再恢复全局状态。
- i18n 同步后检查通过；上游新增的插件工厂更新文本已进入全部语言文件。

验证结果：

- `make test`：通过（主 Go 模块与 relaykit）。
- `bun run typecheck`：通过。
- `bun run test`：通过，144 个测试文件、792 个测试。
- `bun run build:check`：通过。
- `bun run i18n:check`：通过。
- `git diff --check`：通过。
