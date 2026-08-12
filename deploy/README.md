# Molii New API Docker Compose 部署

此目录用于在独立服务器上运行 New API、PostgreSQL 与 Redis。应用镜像从
Docker Hub 拉取，不需要在服务器上安装 Go 或 Bun。

## 首次启动

```sh
cp .env.example .env
openssl rand -hex 24
openssl rand -hex 32
```

将随机值分别写入 `.env` 的数据库密码、Redis 密码和 `SESSION_SECRET`，不要提交
`.env`。检查配置并启动：

```sh
docker compose --env-file .env config
docker compose --env-file .env pull
docker compose --env-file .env up -d
docker compose --env-file .env ps
```

应用默认只监听宿主机 `127.0.0.1:3000`。在 HTTPS 反向代理配置完成前，不要把
`NEW_API_BIND_ADDRESS` 改成公网地址。

## 数据库迁移

`migrate` 服务会在应用启动前按顺序执行以下迁移：

1. `migrations/20260803_molii_postgres.sql`：为已有数据库补充
   `tokens.auto_groups`。
2. `migrations/20260810_async_task_billing_jobs.sql`：创建异步任务计费作业表及其唯一、
   就绪队列和过期租约索引。
3. `migrations/20260811_molii_grok_management_credentials.sql`：为渠道表补充 Molii Grok
   管理访问令牌和管理用户 ID 字段；令牌不会通过渠道 API 回显。
4. `migrations/20260812_default_token.sql`：为 API Key 增加受保护的默认 Key 标记，并保证
   每个用户最多有一条未软删除的默认 Key。

三项迁移都可重复执行；随后 New API 启动时仍会由 GORM AutoMigrate 校验并补充完整
Schema。

手动重跑迁移：

```sh
docker compose --env-file .env run --rm migrate
```

可用一次性的 PostgreSQL 15 容器运行计费作业迁移契约测试（不会连接部署数据库）：

```sh
./migrations/20260810_async_task_billing_jobs_test.sh
./migrations/20260811_molii_grok_management_credentials_test.sh
./migrations/20260812_default_token_test.sh
```

升级已有环境前，建议先备份：

```sh
docker compose --env-file .env exec -T postgres \
  sh -ec 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > new-api.backup.dump
```

## 更新镜像

将 `.env` 中的 `NEW_API_IMAGE` 设置为交付的不可变 `sha-*` 标签，然后运行：

```sh
docker compose --env-file .env pull new-api
docker compose --env-file .env up -d
docker compose --env-file .env ps
```

## 配置 HTTPS

反向代理可用后，将 `.env` 至少调整为：

```env
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://aigc.claudeye.com
TRUSTED_PROXIES=<实际反向代理的 IP 或 CIDR>
```

登录管理后台，在系统设置中将“服务器地址”设置为
`https://aigc.claudeye.com`。该数据库配置会用于 OAuth/支付回调及视频签名播放地址；
它不是环境变量。

如果独立前端与 API 不同 Origin，再设置精确的
`DASHBOARD_CORS_ALLOWED_ORIGINS`。以上 Origin 配置不支持通配符。
