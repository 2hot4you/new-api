# rc.26 PostgreSQL 额度列迁移预案

Molii 开发环境使用 PostgreSQL。rc.26 在应用迁移前强制要求用户钱包相关列为
`bigint`，因此部署新二进制前必须先完成以下独立数据库变更。当前升级任务不连接、
不读取也不修改真实数据库。

## 1. 只读预检

```sql
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'users'
  AND column_name IN ('quota', 'used_quota', 'aff_quota', 'aff_history')
ORDER BY column_name;
```

四列都应存在。旧环境通常显示 `integer`，目标类型为 `bigint`。

## 2. 备份与维护窗口

- 先完成可恢复的数据库快照或逻辑备份。
- 暂停所有 Molii 应用实例和消费/轮询 worker，避免迁移期间写入。
- 确认没有长事务占用 `users` 表。

## 3. 执行迁移

```sql
BEGIN;

ALTER TABLE users
  ALTER COLUMN quota TYPE bigint USING quota::bigint,
  ALTER COLUMN used_quota TYPE bigint USING used_quota::bigint,
  ALTER COLUMN aff_quota TYPE bigint USING aff_quota::bigint,
  ALTER COLUMN aff_history TYPE bigint USING aff_history::bigint;

COMMIT;
```

## 4. 迁移后确认

重新执行只读预检，确认四列均为 `bigint`，然后只启动一个 rc.30 候选实例观察
启动迁移日志。不要设置 `SKIP_64BIT_QUOTA_SCHEMA_CHECK=true`；该开关仅用于已由
外部机制确认 schema 的特殊环境，不是正常升级路径。

## 5. 回滚边界

在应用仍可能写入超过 32 位范围的额度后，不得把列直接缩回 `integer`。代码回滚
可以继续读取 `bigint`，数据库类型保持 `bigint` 即可。
