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

## 独立 Claudeye 生产环境（CI/CD）

此节适用于 `production-claudeye`（上海 `claudeye.com`）及
`production-model-claudeye`（洛杉矶 `model.claudeye.com`）。使用独立基础设施
`/opt/claudeye/infra/compose.json`，不要使用本文开头的一体化 Compose 启动方式。
两个站点分别运行在独立服务器；路径相同不代表共享数据。

`prepare-runtime.py --site ENV` 只在对应主机以 root 运行，保留已有
`infra/app.env` 四项凭据，创建全新 `/opt/claudeye/production/.env.runtime`。
已有目录拒绝覆盖。文件归 `claudeye-deploy`、权限0600；不要输出文件内容。
部署账号通过 Docker 组操作 Docker，其权限等同宿主机管理员，应仅用于可信 CI。

应用使用 `docker-compose.local-db.yml`，由 CI 上传为应用目录的
`docker-compose.yml`。应用 project 与数据库 project 分离，仅管理 new-api 服务；
外接各站独立的 infra backend 网络，同时保留独立出口网络以访问上游 API。
应用端口仅发布到 `127.0.0.1:3000`，站点反代此地址。应用部署不会重建数据库、
删除基础设施卷或使用 `--remove-orphans`。新站应用以1000:1000运行，data/logs/certs
需允许该用户访问。Docker daemon 代理与应用出站隔离，不向应用注入代理变量。

GitHub Environment 除已有 SSH secrets、DEPLOY_DIR、DEPLOY_HEALTH_URL、
DEPLOY_SITE_DOMAIN 外，每个新站必须独立填写以下公开构建变量：

| 变量 | 内容 |
| --- | --- |
| VITE_SITE_PROFILE | claudeye |
| VITE_SITE_TITLE | 对应站点标题 |
| VITE_SITE_DESCRIPTION | 对应站点简介 |
| VITE_SITE_LOGO | 仓库 public 资源路径或 HTTPS 资源地址 |
| VITE_SITE_FAVICON | favicon 路径或 HTTPS 地址 |
| VITE_SITE_APPLE_TOUCH_ICON | Apple Touch 图标路径或 HTTPS 地址 |
| VITE_SITE_BANNER_BRAND | 对应品牌显示名称 |
| VITE_SITE_DEFAULT_FONT | sans 或 serif |

缺少品牌参数时部署会失败，避免借用 Molii/iXiaozu 品牌。构建测试中的占位资源
仅用于测试，不作为上线配置。服务器运行配置、数据库数据与密钥均不进入 Actions。

发布顺序：功能分支先经测试合 develop，仅部署开发站，由用户验收。之后经明确
确认合 main。main 应用自动部署及应用 `all-production` 均包含四个生产环境：
production-molii、production-ixiaozu、production-claudeye（上海）、
production-model-claudeye（洛杉矶）。仍可手动选择单个环境发布。
纯文档变更继续由独立文档工作流处理，不重启应用；其他应用变更按原有路径规则触发。
develop 始终只自动部署 development，不发布任何生产站点。
生产工作流仅允许 main 上该次触发的固定 SHA，
不接受候选分支 source_ref。验证、构建和部署使用同一解析后的 SHA。

首次应用启动会初始化应用数据库，应先确认已有备份恢复演练及最新备份。应用
部署仍是单容器替换，会有短暂中断。镜像回滚不会回滚数据库迁移；发布版本必须
保持数据库向后兼容。新站首次失败没有旧应用可回滚，需修复后重新明确部署。
不可为调试而手工清库或删除数据库卷。

两个新站的已确认临时品牌配置见 `deploy/claudeye-brand.json`：名称均为小写
`claudeye`，使用同一套仓库内 C 字占位 Logo、32px favicon 与180px Apple Touch
图标。它们仅用于首次搭建验收，可替换；品牌相同不改变两站数据库和运行配置
隔离。此 JSON 仅含公开构建变量，不含密钥。
