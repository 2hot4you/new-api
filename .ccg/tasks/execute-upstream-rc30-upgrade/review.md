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

## v1.0.0-rc.29

- 接收上游 `prefill_groups.name` PostgreSQL 遗留全局唯一约束迁移：仅自动处理已知旧约束/索引，未知对象会阻断启动并保留原状。
- 将该迁移置于 `AutoMigrate` 之前，避免 GORM 在旧约束仍存在时失败。
- 接收上游 MySQL/PostgreSQL GORM 驱动配套升级。
- 移除已被上游撤销的并发 `migrateDBFast`，保留串行迁移中的全部 Molii 表和迁移后初始化步骤。

验证结果：

- SQLite 迁移聚焦测试：通过，包含重复执行的幂等性检查。
- `make test`：通过（主 Go 模块与 relaykit；未配置的 MySQL/PostgreSQL 外部集成测试按设计跳过）。
- `git diff --check`：通过。

上线前阻断项：

- 本任务约束禁止连接真实数据库，因此尚未执行新项目规范要求的 SQLite/MySQL/PostgreSQL 三数据库新装、从上一发布版本升级及二次启动矩阵。不能据此宣称 rc.29 数据库迁移已完成生产兼容验证；必须在隔离的候选环境补跑后才能发布。

## v1.0.0-rc.30

- 接收上游 `tokens.key` PostgreSQL 遗留唯一约束迁移：只转换已知旧约束，拒绝未知或非标准定义，并保留无关复合约束与部分索引。
- 将 token key 迁移置于 prefill group 迁移及 `AutoMigrate` 之前。
- 接收用量统计 quota 保留修复：RPM/TPM 使用独立结果对象扫描，避免第二次扫描把已汇总 quota 清零。
- 新增 Molii 回归测试，覆盖 quota、RPM、TPM 同时返回的行为。

验证结果：

- SQLite token key 迁移聚焦测试：通过，包含重复执行的幂等性检查。
- quota 汇总回归测试：通过。
- `make test`：通过（主 Go 模块与 relaykit；未配置的 MySQL/PostgreSQL 外部集成测试按设计跳过）。

上线前阻断项与 rc.29 相同：必须在隔离候选环境完成真实三数据库迁移矩阵后才能发布。
