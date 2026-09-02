# Molii 升级到 new-api v1.0.0-rc.30

本文用于把 Molii 的 rc.30 候选分支部署到隔离环境。不要直接在生产数据库首次执行迁移。

## 发布前提

- 对应用数据库和对象存储配置完成可恢复备份。
- 停止所有 API 实例、异步任务轮询器和结算 worker，避免迁移期间写入。
- 在与生产同引擎、同主版本的隔离数据库副本上完成本文矩阵。
- 确认候选提交包含上游标签 `v1.0.0-rc.30`：

```bash
git merge-base --is-ancestor v1.0.0-rc.30 HEAD
```

## rc.26：钱包额度列改为 64 位

PostgreSQL 必须确认 `users` 表以下四列均为 `bigint`：

```sql
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'users'
  AND column_name IN ('quota', 'used_quota', 'aff_quota', 'aff_history')
ORDER BY column_name;
```

如仍为 `integer`，在应用停机后执行：

```sql
BEGIN;
ALTER TABLE users
  ALTER COLUMN quota TYPE bigint USING quota::bigint,
  ALTER COLUMN used_quota TYPE bigint USING used_quota::bigint,
  ALTER COLUMN aff_quota TYPE bigint USING aff_quota::bigint,
  ALTER COLUMN aff_history TYPE bigint USING aff_history::bigint;
COMMIT;
```

不要使用 `SKIP_64BIT_QUOTA_SCHEMA_CHECK=true` 代替迁移。MySQL 也必须确认对应列已是 `BIGINT`。

## rc.29 与 rc.30：PostgreSQL 遗留唯一约束

候选版本会在 GORM `AutoMigrate` 前处理两类已知旧定义：

- rc.29：`prefill_groups.name` 的遗留全局唯一约束或索引。
- rc.30：`tokens.key` 的遗留全局唯一约束或索引。

迁移只接受上游明确识别的旧结构。若发现未知名称、复合约束、部分索引或非标准定义，启动会安全失败并保留原状。此时不要手工删除对象；先保存启动日志和以下只读结果，再评估具体 DDL：

```sql
SELECT schemaname, tablename, indexname, indexdef
FROM pg_indexes
WHERE schemaname = current_schema()
  AND tablename IN ('prefill_groups', 'tokens')
ORDER BY tablename, indexname;

SELECT conrelid::regclass AS table_name, conname, pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid IN ('prefill_groups'::regclass, 'tokens'::regclass)
ORDER BY table_name, conname;
```

## 必跑数据库矩阵

SQLite、MySQL、PostgreSQL 每种引擎均执行：

1. 全新空库启动候选版本。
2. 从当前线上版本的脱敏数据库副本启动候选版本。
3. 在同一数据库上停止并再次启动候选版本，确认迁移幂等。
4. 检查用户、令牌、分组、渠道、任务、日志和模型元数据的关键行数及唯一性。
5. 验证创建/编辑 API Key、单分组与有序跨分组路由、文本请求、图片请求、异步任务提交与终态结算。

任一启动出现迁移错误、额度列类型不符、未知约束或数据计数异常，都应阻断发布。

## 单实例冒烟

数据库矩阵通过后，只启动一个候选实例并检查：

- `/api/status`、登录和密码传输加密公钥。
- API Key 创建、编辑、模型/IP 限制及跨分组顺序。
- 普通文本、GPT Image 2、StarAI、Molii Grok 和 Task Plugin 请求。
- 临时素材上传、刷新、签名代理和 COS 生命周期。
- 用量日志、quota/RPM/TPM、预扣费、退款和终态真实结算。
- 排行榜、模型广场、管理端和 Docusaurus 文档入口。

观察确认后再逐步扩容。代码回滚时保留 rc.26 已扩展的 `bigint` 列，不要缩回 32 位。
