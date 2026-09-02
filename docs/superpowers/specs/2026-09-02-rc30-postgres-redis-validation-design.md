# rc.30 PostgreSQL 与 Redis 升级验收设计

## 目标

为 Molii 的 `upgrade/rc30-dev` 候选分支建立只针对实际运行栈 PostgreSQL 15 与 Redis 7 的持续集成门禁，消除 controller 组合 Race Detector 中由跨测试异步任务泄漏引起的竞态，并在可恢复备份后直接使用 development 环境完成迁移与单实例冒烟，最终合入 `develop`。

## 范围与边界

- SQLite 与 MySQL 不作为发布条件，也不进入新增 CI 矩阵。
- CI 服务容器使用空 PostgreSQL/Redis，不读取任何云环境秘密。
- development 云 PostgreSQL 只在服务器端备份和恢复；备份内容不进入 Git 或聊天。
- development Redis 不清库。验证只创建带随机命名空间和短 TTL 的探针键，并在结束时删除。
- 生产环境不在本任务范围内。

## CI 设计

后端 CI 增加 PostgreSQL 15 与 Redis 7 服务容器及健康检查。现有完整 Go 测试继续执行，同时通过 `TEST_POSTGRES_DSN`、`MARKETPLACE_ORDER_POSTGRES_TEST_DSN`、`MODEL_MARKETPLACE_POSTGRES_TEST_DSN` 和新增的 `TEST_REDIS_DSN` 激活真实 PostgreSQL/Redis 集成测试。

PostgreSQL 集成测试覆盖 rc.29 `prefill_groups.name`、rc.30 `tokens.key`、用户会话字段、模型广场排序/回填以及完整 schema 的首次迁移与二次迁移。Redis 集成测试使用唯一键验证连接、写入、读取、TTL、原子增量和删除，不使用 `FLUSHDB`/`FLUSHALL`。

## Race Detector 修复

根因不是 StarAI 或 Molii Grok 本身，而是 request-scoped 的 fire-and-forget 任务由 `gopool.Go` 启动后跨越测试边界，随后读取已经被下一项测试替换或关闭的全局 Redis/DB 句柄。

新增一个后台任务调度边界：生产模式继续委托 `gopool.Go`；测试可在进程启动前切换为同步执行。仅把会访问这些全局依赖的请求尾部指标、额度通知和管理审计接入该边界。controller 的 `TestMain` 在任何测试运行前启用同步模式，运行期间不再切换，从而避免调度器自身竞态，并确保每项请求在测试返回前完成副作用。

## development 备份与迁移

通过现有 `molii-deploy` SSH 凭据进入服务器。先确认 development 容器、镜像 digest、环境文件权限和 PostgreSQL 可达性，再创建带 UTC 时间戳的 custom-format 备份。备份必须同时满足：`pg_dump` 成功、文件非空、`pg_restore --list` 可解析、SHA-256 已记录。

备份通过后停止 development 应用容器，执行四个钱包字段的 `bigint` 预检/迁移，并只读检查 `prefill_groups` 与 `tokens` 约束。随后启动一个 rc.30 候选实例，检查迁移日志和健康端点；再重启同一实例验证幂等性。失败时停止候选并恢复先前不可变镜像；如 schema 需要恢复，则使用已校验备份在明确确认后执行。

## 冒烟与发布门禁

冒烟覆盖登录、API Key、跨分组、文本请求、GPT Image 2、StarAI/Seedance、Molii Grok、临时素材、COS、用量日志及真实结算。涉及付费上游的请求只使用 development 测试账户并控制为最小数量。

只有 CI、完整 Race Detector、备份、首次/二次迁移、公共健康检查和冒烟全部通过，才把候选分支合入 `develop` 并推送。任何数据库或计费异常都会阻断合并。

## 审查限制

项目要求的 antigravity 与 Claude 外部分析已并行调用，但当前主机缺少 `~/.claude/bin/codeagent-wrapper`，两路均返回 127。该限制记录在最终审查中，不以本地测试替代“已完成外部双模型审查”的声明。
