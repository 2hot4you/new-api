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

## 跨版本最终审查

代码与能力审计：

- `v1.0.0-rc.30` 已成为当前分支祖先，rc.24 至 rc.30 均保留独立合并检查点。
- 保留 Molii 渠道编号：StarAI=61、Molii Grok=62、Task Plugin=63、Dummy=64。
- 保留有序跨分组路由、StarAI/Molii Grok 原生适配器、任务插件双栈、临时素材、COS、GPT Image 2、任务真实结算及用量日志能力。
- 上游删除的旧任务适配器均已由 JavaScript 任务插件替代，没有删除 Molii 原生适配器。

最终验证：

```text
make test                                  PASS
go vet ./...                               PASS
bun run typecheck                          PASS
bun run test                               PASS (144 files, 792 tests)
bun run build:check                        PASS
bun run i18n:check                         PASS
bun run format:plugins:check               PASS
bun run lint:plugins                       PASS (7 条上游 preserve-caught-error warning)
git diff --check                           PASS
```

数据库与运行时验证：

- 使用显式临时目录和全新 SQLite 数据库构建、启动 rc.30 候选二进制，`/api/status` 与登录加密公钥接口正常。
- 对同一 SQLite 数据库执行第二次启动，迁移幂等且登录加密密钥没有重复生成。
- 未连接开发或生产数据库；MySQL/PostgreSQL 的新装、上一版本升级和二次启动必须在隔离副本上完成后才能发布。

并发检查：

- `go test -race ./model ./middleware ./service` 通过。
- `go test -race ./relay ./router` 通过。
- Molii Grok、StarAI、Task Plugin 协议及计费别名配置拆分运行均通过 `-race`。
- `controller` 组合运行暴露既有测试隔离竞态：前一个测试启动的异步指标/通知任务尚未结束，后一个测试已替换全局 Redis/DB 测试句柄。该竞态不属于 rc.30 合并路径，但完整 `controller -race` 不能标记为通过，后续应单独重构异步任务测试生命周期。

审查工具限制：

- CCG 要求的 antigravity 与 Claude 双模型外部审查未能执行，因为当前主机不存在 `~/.claude/bin/codeagent-wrapper`，`PATH` 中也没有该工具。已记录为环境限制，不能宣称已完成外部双模型审查。

结论：代码集成候选可交付到隔离数据库验收阶段；在真实 MySQL/PostgreSQL 迁移矩阵通过前，不具备生产发布结论。
