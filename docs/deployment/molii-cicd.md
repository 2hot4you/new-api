# Molii 三环境、双生产站点 CI/CD 操作手册

本仓库使用一条验收链路和两个相互独立的生产站点：

| Git 分支 | GitHub Environment | 站点 | 应用端口 | 服务器目录 | 品牌 |
| --- | --- | --- | --- | --- | --- |
| `develop` | `development` | `dev.molii.co` | `3010` | `/opt/molii/development` | Molii |
| `main` | `production-molii` | `molii.co` | `3000` | `/opt/molii/production` | Molii |
| `main` | `production-ixiaozu` | `aigc.ixiaozu.cn` | `3000` | `/opt/ixiaozu/production` | iXiaozu |

`main` 的两个生产任务使用矩阵并行发布，互不共享服务器连接、数据库、Redis、用户、渠道、运行时密钥、应用镜像或文档品牌产物。一个生产目标失败不会取消另一个；Actions 汇总会明确显示 `success`、`partial_success` 或 `failure`。

## 一、发布顺序

1. 本地开发和测试。
2. 合并并推送到 `develop`，自动发布 `dev.molii.co`。
3. 在开发站验收数据库迁移、Redis、登录、渠道和核心 API。
4. 将同一个已验收提交从 `develop` 合并到 `main`。
5. 推送 `main` 后，分别审批并发布 `molii.co` 与 `aigc.ixiaozu.cn`。

不要从未经开发环境验收的功能分支直接部署生产。

## 二、服务器准备

每台服务器需要 Ubuntu、Docker、Docker Compose v2、`curl`、`flock`、`rsync`、`tar` 和 `sha256sum`。部署账号建议统一为无密码、无 sudo 的 `molii-deploy`，只加入 `docker` 组。

Molii 服务器创建：

```bash
sudo adduser --disabled-password --gecos '' molii-deploy
sudo usermod -aG docker molii-deploy
sudo install -d -m 0750 -o molii-deploy -g molii-deploy \
  /opt/molii/development /opt/molii/development/data /opt/molii/development/logs /opt/molii/development/certs \
  /opt/molii/production /opt/molii/production/data /opt/molii/production/logs /opt/molii/production/certs
```

iXiaozu 服务器创建：

```bash
sudo adduser --disabled-password --gecos '' molii-deploy
sudo usermod -aG docker molii-deploy
sudo install -d -m 0750 -o molii-deploy -g molii-deploy \
  /opt/ixiaozu/production /opt/ixiaozu/production/data /opt/ixiaozu/production/logs \
  /opt/ixiaozu/production/certs
```

已有账号时跳过 `adduser`。确认账号能运行 `docker info` 和 `docker compose version`，不要把 Docker socket 改成全局可写。

## 三、SSH 部署凭据

三个 GitHub Environment 可以使用不同 SSH 主机和私钥。建议每台服务器生成独立的 GitHub Actions 部署密钥，并人工核对服务器 Ed25519 主机指纹。

Environment Secrets：

| Secret | 说明 |
| --- | --- |
| `DEPLOY_SSH_HOST` | 当前目标服务器地址 |
| `DEPLOY_SSH_PORT` | SSH 端口 |
| `DEPLOY_SSH_USER` | `molii-deploy` |
| `DEPLOY_SSH_PRIVATE_KEY` | 当前服务器对应的完整 Ed25519 私钥 |
| `DEPLOY_SSH_KNOWN_HOSTS` | 人工核验后的当前服务器主机记录 |
| `TELEGRAM_BOT_TOKEN` | 可选，发布通知机器人 |
| `TELEGRAM_CHAT_ID` | 可选，发布通知目标 |

不要把数据库、Redis、`SESSION_SECRET` 或 `CRYPTO_SECRET` 放入 GitHub；它们只存在于各自服务器的 `.env.runtime`。

## 四、独立运行时配置

每个目标各自创建权限为 `0600`、所有者为 `molii-deploy` 的 `.env.runtime`：

- Development：`/opt/molii/development/.env.runtime`
- Molii Production：`/opt/molii/production/.env.runtime`
- iXiaozu Production：`/opt/ixiaozu/production/.env.runtime`

模板：

```dotenv
SQL_DSN=postgresql://APP_USER:URL_ENCODED_PASSWORD@POSTGRES_HOST:5432/APP_DATABASE?sslmode=require
REDIS_CONN_STRING=rediss://REDIS_USER:URL_ENCODED_PASSWORD@REDIS_HOST:6379/0
REDIS_TLS_CA_FILE=/app/certs/redis-ca.pem
SESSION_SECRET=AT_LEAST_32_RANDOM_CHARACTERS
CRYPTO_SECRET=ANOTHER_32_RANDOM_CHARACTERS
TZ=Asia/Shanghai
ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
NODE_NAME=TARGET_SPECIFIC_NODE_NAME
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://TARGET_DOMAIN
```

三套配置必须使用不同 PostgreSQL 数据库/账号、Redis 实例或逻辑隔离、会话密钥和加密密钥。数据库与 Redis 白名单仅放行对应服务器，用户和渠道配置不会在生产站之间同步。

使用云数据库私有 CA 时，将证书放入目标目录的 `certs/`，容器会将其只读挂载到 `/app/certs`。PostgreSQL DSN 应使用 `sslmode=verify-full&sslrootcert=/app/certs/<certificate>.pem`；Redis 必须使用 `rediss://`，并通过 `REDIS_TLS_CA_FILE=/app/certs/<certificate>.pem` 指定 CA。应用不会跳过服务端身份校验，配置的证书缺失或无法解析时会拒绝启动。

## 五、反向代理与文档目录

应用反向代理：

| 域名 | 上游 |
| --- | --- |
| `dev.molii.co` | `http://127.0.0.1:3010` |
| `molii.co` | `http://127.0.0.1:3000` |
| `aigc.ixiaozu.cn` | `http://127.0.0.1:3000`（位于独立服务器） |

每个域名开启 HTTPS 和 HTTP→HTTPS。应用端口不得向公网开放。

Docusaurus 构建产物由独立工作流发布到对应站点根目录下的 `docs/`：

- `/opt/1panel/www/sites/dev.molii.co/index/docs`
- `/opt/1panel/www/sites/molii.co/index/docs`
- `/opt/1panel/www/sites/aigc.ixiaozu.cn/index/docs`

反向代理需要让 `/docs` 与 `/docs/` 返回 308 到 `/docs/quick-start`，并优先从静态 `docs/` 目录提供内容。文档脚本会校验压缩包、目标域名、站点标题和公网页面；失败时恢复上一份静态快照。

## 六、GitHub Environments

在 `Settings → Environments` 创建名称完全一致的：

- `development`
- `production-molii`
- `production-ixiaozu`

建议两个 production Environment 都启用 required reviewer。每个 Environment 单独配置上一节的 SSH Secrets 和以下 Variables。

### 6.1 应用 Variables

| Variable | development | production-molii | production-ixiaozu |
| --- | --- | --- | --- |
| `DEPLOY_DIR` | `/opt/molii/development` | `/opt/molii/production` | `/opt/ixiaozu/production` |
| `DEPLOY_HEALTH_URL` | `https://dev.molii.co/api/status` | `https://molii.co/api/status` | `https://aigc.ixiaozu.cn/api/status` |
| `DEPLOY_SITE_DOMAIN` | `dev.molii.co` | `molii.co` | `aigc.ixiaozu.cn` |
| `VITE_SITE_PROFILE` | `molii` | `molii` | `ixiaozu` |
| `VITE_SITE_TITLE` | Molii 站点标题 | Molii 站点标题 | iXiaozu 站点标题 |
| `VITE_SITE_DESCRIPTION` | Molii 描述 | Molii 描述 | iXiaozu 描述 |
| `VITE_SITE_LOGO` | 公共 HTTPS Logo URL | 公共 HTTPS Logo URL | iXiaozu 公共 HTTPS Logo URL |
| `VITE_SITE_FAVICON` | 公共 HTTPS Favicon URL | 公共 HTTPS Favicon URL | iXiaozu 公共 HTTPS Favicon URL |
| `VITE_SITE_APPLE_TOUCH_ICON` | 公共 HTTPS 图标 URL | 公共 HTTPS 图标 URL | iXiaozu 公共 HTTPS 图标 URL |
| `VITE_SITE_BANNER_BRAND` | `Molii` | `Molii` | 首页 Banner 展示名称 |
| `VITE_SITE_DEFAULT_FONT` | `serif` 或 `sans` | `serif` 或 `sans` | `serif` 或 `sans` |

Logo URL 会在应用构建期写入对应镜像。首页彩色品牌字按 Unicode 字符顺序交替使用粉色和蓝色，不限制字数，也不会删除现有彩色组件。

### 6.2 文档 Variables

| Variable | development | production-molii | production-ixiaozu |
| --- | --- | --- | --- |
| `DOCS_ENV` | `development` | `production` | `production` |
| `DOCS_SITE_URL` | `https://dev.molii.co` | `https://molii.co` | `https://aigc.ixiaozu.cn` |
| `DOCS_API_BASE_URL` | 同站点 URL | 同站点 URL | 同站点 URL |
| `DOCS_BRAND_ID` | `molii` | `molii` | `ixiaozu` |
| `DOCS_SITE_TITLE` | Molii 文档标题 | Molii 文档标题 | iXiaozu 文档标题 |
| `DOCS_TAGLINE` | Molii 文档描述 | Molii 文档描述 | iXiaozu 文档描述 |
| `DOCS_NAVBAR_TITLE` | `Molii` | `Molii` | iXiaozu 导航名称 |
| `DOCS_LOGO_PATH` | `img/brand/logo.png` | `img/brand/logo.png` | `img/brand/logo.png` |
| `DOCS_FAVICON_PATH` | `img/brand/favicon.png` | `img/brand/favicon.png` | `img/brand/favicon.png` |
| `DOCS_SOCIAL_IMAGE_PATH` | `img/brand/social.png` | `img/brand/social.png` | `img/brand/social.png` |
| `DOCS_DEFAULT_FONT` | `serif` 或 `sans` | `serif` 或 `sans` | `serif` 或 `sans` |
| `DOCS_LOGO_SOURCE_URL` | 公共 HTTPS 图片 URL | 公共 HTTPS 图片 URL | iXiaozu 公共 HTTPS 图片 URL |
| `DOCS_FAVICON_SOURCE_URL` | 公共 HTTPS 图片 URL | 公共 HTTPS 图片 URL | iXiaozu 公共 HTTPS 图片 URL |
| `DOCS_SOCIAL_IMAGE_SOURCE_URL` | 公共 HTTPS 图片 URL | 公共 HTTPS 图片 URL | iXiaozu 公共 HTTPS 图片 URL |

生产文档素材源必须为 HTTPS，不允许跳转、凭据、超过 2 MiB 的文件或含脚本/外链的危险 SVG。素材只进入当前目标的构建产物，不提交 Git，也不会在两个生产站间复用。

Development 可另外配置以下 Environment Secrets 启用 Algolia；生产构建不会读取这些值：

- `DOCS_ALGOLIA_APP_ID`
- `DOCS_ALGOLIA_SEARCH_API_KEY`
- `DOCS_ALGOLIA_INDEX_NAME`

## 七、首次发布与验收

推送 `develop` 后检查：

```bash
curl --fail --silent --show-error https://dev.molii.co/api/status
curl --fail --silent --show-error https://dev.molii.co/docs/quick-start >/dev/null
```

开发环境验收通过后合并到 `main`。两个生产目标会分别等待各自 Environment 审批；可以先批准一个，再批准另一个。发布后检查：

```bash
curl --fail --silent --show-error https://molii.co/api/status
curl --fail --silent --show-error https://molii.co/docs/quick-start >/dev/null
curl --fail --silent --show-error https://aigc.ixiaozu.cn/api/status
curl --fail --silent --show-error https://aigc.ixiaozu.cn/docs/quick-start >/dev/null
```

分别验证登录、用户、渠道、密钥、账单和 Redis 会话，确认两个生产站的数据互不可见。镜像使用 GHCR digest 发布，回滚不会依赖可变标签。

## 八、故障排查与恢复

```bash
cd /opt/molii/development
docker compose --env-file .deploy.env ps
docker logs --tail 100 molii-development
```

其他目标分别替换为：

- `/opt/molii/production`、`molii-production`
- `/opt/ixiaozu/production`、`ixiaozu-production`

应用部署在容器或公网健康检查失败时自动恢复上一个镜像。文档部署在页面或品牌标记检查失败时自动恢复上一份文件快照。第一次部署没有旧版本时会保留失败现场供排查。

不要删除 `data/`、`logs/`、`.env.runtime`，也不要运行 `docker system prune --volumes`。

## 九、持续健康监控

在 Uptime Kuma 中分别创建三个 HTTPS Monitor：

- `https://dev.molii.co/api/status`
- `https://molii.co/api/status`
- `https://aigc.ixiaozu.cn/api/status`

要求 HTTP 200 且正文包含 `"success":true`。建议间隔 60 秒、连续失败 3 次告警，并配置故障与恢复通知。
