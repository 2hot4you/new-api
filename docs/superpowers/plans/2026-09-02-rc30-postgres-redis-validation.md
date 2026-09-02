# rc.30 PostgreSQL 与 Redis 升级验收实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 rc.30 候选建立 PostgreSQL/Redis CI 和稳定 Race Detector，并在备份后完成 development 迁移、冒烟、合并与推送。

**Architecture:** GitHub Actions 使用隔离 PostgreSQL 15/Redis 7 服务激活真实集成测试；request-scoped 异步副作用通过统一调度边界在生产异步、controller 测试同步。development 通过服务器端可验证备份、停写、单实例迁移和二次启动完成验收。

**Tech Stack:** Go 1.26、GORM、PostgreSQL 15、Redis 7、GitHub Actions、Docker Compose、Bash。

**Spec:** `docs/superpowers/specs/2026-09-02-rc30-postgres-redis-validation-design.md`

## Global Constraints

- 仅 PostgreSQL 15 与 Redis 7 是新增发布门禁。
- 不读取 CI 之外的秘密，不清空 development Redis。
- dev 数据库操作前必须生成并校验可恢复备份。
- 新行为严格执行 Red-Green-Refactor。
- 数据库、计费或结算异常阻断合并推送。

---

### Task 1: 后台任务测试边界

**Files:**
- Create: `common/background_task.go`
- Create: `common/background_task_test.go`
- Create: `controller/main_test.go`
- Modify: `controller/relay.go`
- Modify: `middleware/audit.go`
- Modify: `service/quota.go`

**Interfaces:**
- Produces: `common.RunBackgroundTask(func())` 和进程启动前使用的同步测试配置入口。
- Consumes: `gopool.Go` 作为生产调度器。

- [x] **Step 1: 写后台任务同步/异步行为测试，并记录当前缺少接口的失败。**
- [x] **Step 2: 运行 `go test ./common -run TestBackgroundTask -count=1`，确认按预期失败。**
- [x] **Step 3: 实现最小调度边界并接入 relay 指标、额度通知和管理审计。**
- [x] **Step 4: 增加 controller `TestMain`，在测试进程启动前固定为同步模式。**
- [x] **Step 5: 运行 `go test -race ./controller -count=1` 和相关包测试，确认组合竞态消失。**
- [x] **Step 6: 提交独立竞态修复检查点。**

### Task 2: PostgreSQL 与 Redis CI

**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: `common/redis_integration_test.go`
- Create: `model/postgres_migration_integration_test.go`

**Interfaces:**
- Consumes: `TEST_POSTGRES_DSN`、`MARKETPLACE_ORDER_POSTGRES_TEST_DSN`、`MODEL_MARKETPLACE_POSTGRES_TEST_DSN`、`TEST_REDIS_DSN`。
- Produces: PostgreSQL 首次/二次 schema 迁移和 Redis 非破坏性真实实例测试。

- [x] **Step 1: 写 gated PostgreSQL/Redis 集成测试，先在未实现完整迁移/环境时观察预期跳过或失败。**
- [x] **Step 2: 为 CI 后端 job 加入 PostgreSQL 15、Redis 7 服务和健康检查。**
- [x] **Step 3: 使用本地 Docker PostgreSQL/Redis 运行新增测试。**
- [x] **Step 4: 运行完整 `make test`、workflow YAML 解析和 `git diff --check`。**
- [x] **Step 5: 提交 CI 检查点。**

### Task 3: development 备份与迁移演练

**Files:**
- Modify: `.ccg/tasks/validate-rc30-postgres-redis/review.md`

**Interfaces:**
- Consumes: `/opt/molii/development/.env.runtime`、当前容器镜像 digest、云 PostgreSQL/Redis。
- Produces: 已校验的 `pg_dump -Fc`、SHA-256、迁移前后检查结果和回滚引用。

- [x] **Step 1: 通过受保护 GitHub Environment 的现有 SSH 凭据核对并操作 development；工作流运行器不接收数据库 DSN。**
- [x] **Step 2: 在服务器受限目录生成 PostgreSQL custom-format 备份。**
- [x] **Step 3: 以非空检查、`pg_restore --list` 和 SHA-256 校验备份。**
- [ ] **Step 4: 停止 development 应用写入，检查并迁移四个钱包 `bigint` 字段。**
- [ ] **Step 5: 记录 `prefill_groups.name`、`tokens.key` 迁移前约束，启动候选单实例。**
- [x] **Step 6: 验证候选版本首次启动和重复启动；完整 PostgreSQL 测试验证 schema 二次迁移及关键种子行保留。**

### Task 4: 单实例冒烟、合并与推送

**Files:**
- Modify: `.ccg/tasks/validate-rc30-postgres-redis/review.md`
- Modify: `.ccg/tasks/validate-rc30-postgres-redis/task.json`

**Interfaces:**
- Consumes: development rc.30 候选实例。
- Produces: 冒烟证据、最终审查记录及 `develop` 合并提交。

- [ ] **Step 1: 验证健康、登录、API Key 和跨分组。**
- [ ] **Step 2: 对文本及各媒体渠道执行最小真实请求，核对日志、退款和终态结算。**
- [x] **Step 3: 验证临时素材、使用日志、排行榜、模型广场和文档等公开入口均返回 HTTP 200；需鉴权模块保持 HTTP 401。**
- [x] **Step 4: 运行最终后端、前端、Race Detector 和工作树检查。**
- [ ] **Step 5: 更新审查记录并归档 CCG 任务。**
- [ ] **Step 6: 合入 `develop` 并推送；确认 development 自动部署健康。**
