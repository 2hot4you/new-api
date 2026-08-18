# Molii 双环境 CI/CD 操作手册

本手册部署当前仓库的两个环境：

| 环境 | 分支 | 域名 | 1Panel 反向代理 | 服务器目录 |
| --- | --- | --- | --- | --- |
| Production | `main` | `molii.co` | `127.0.0.1:3000` | `/opt/molii/production` |
| Development | `develop` | `dev.molii.co` | `127.0.0.1:3010` | `/opt/molii/development` |

运行时秘密只存放在服务器。GitHub Actions 构建镜像、推送到 GHCR，并通过 SSH 调用仓库内的部署脚本。

## 一、服务器一次性准备

前置条件：Ubuntu 24.04、1Panel、Docker、Docker Compose v2 和 OpenResty 已安装。以下命令在服务器管理员终端执行。

创建不带密码的专用部署用户，并授权它管理 Docker：

```bash
sudo adduser --disabled-password --gecos '' molii-deploy
sudo usermod -aG docker molii-deploy
sudo install -d -m 0750 -o molii-deploy -g molii-deploy /opt/molii/production
sudo install -d -m 0750 -o molii-deploy -g molii-deploy /opt/molii/development
sudo install -d -m 0750 -o molii-deploy -g molii-deploy /opt/molii/production/data /opt/molii/production/logs
sudo install -d -m 0750 -o molii-deploy -g molii-deploy /opt/molii/development/data /opt/molii/development/logs
```

重新登录该用户后验证 Docker 权限：

```bash
sudo -iu molii-deploy docker version
sudo -iu molii-deploy docker compose version
```

如果命令仍提示权限不足，确认用户组后重新登录，不要把 Docker socket 改成全局可写：

```bash
id molii-deploy
getent group docker
```

## 二、配置部署 SSH 密钥

在可信电脑生成只用于 GitHub Actions 的 Ed25519 密钥，不要为私钥设置交互式口令：

```bash
ssh-keygen -t ed25519 -C 'github-actions-molii-deploy' -f ./molii-deploy-github
```

将 `molii-deploy-github.pub` 的单行内容安装到服务器：

```bash
sudo install -d -m 0700 -o molii-deploy -g molii-deploy /home/molii-deploy/.ssh
sudo install -m 0600 -o molii-deploy -g molii-deploy ./molii-deploy-github.pub /home/molii-deploy/.ssh/authorized_keys
```

确认新密钥可以登录后，再将服务器 SSH 端口限制为可信来源。不要关闭当前管理员会话，直到新登录验证成功。

获取 GitHub `DEPLOY_SSH_KNOWN_HOSTS` 前，先在服务器查看真实主机密钥指纹：

```bash
sudo ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

然后在可信电脑执行以下命令，并人工核对指纹。将完整输出保存为 GitHub Secret；不要在 Actions 中动态运行 `ssh-keyscan`：

```bash
ssh-keyscan -p 22 -t ed25519 YOUR_SERVER_HOST
```

## 三、创建服务器运行时环境文件

先创建空文件并限制权限：

```bash
sudo install -m 0600 -o molii-deploy -g molii-deploy /dev/null /opt/molii/production/.env.runtime
sudo install -m 0600 -o molii-deploy -g molii-deploy /dev/null /opt/molii/development/.env.runtime
```

分别生成两套独立密钥，并保存到密码管理器：

```bash
openssl rand -hex 32
openssl rand -hex 32
openssl rand -hex 32
openssl rand -hex 32
```

前两条用于 Production 的 `SESSION_SECRET`、`CRYPTO_SECRET`，后两条用于 Development。切勿把输出发到聊天、提交到 Git 或粘贴进 GitHub Actions 日志。

以仓库中的示例为基础填写服务器文件：

- Production：`deploy/env/production.env.example`
- Development：`deploy/env/development.env.example`

Production 结构：

```dotenv
SQL_DSN=postgresql://molii_prod_app:URL_ENCODED_PASSWORD@POSTGRES_HOST:5432/molii_prod?sslmode=require
REDIS_CONN_STRING=redis://REDIS_USER:URL_ENCODED_PASSWORD@MOLIICO_REDIS_HOST:6379/0
SESSION_SECRET=PRODUCTION_RANDOM_HEX
CRYPTO_SECRET=PRODUCTION_RANDOM_HEX
TZ=Asia/Shanghai
ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
NODE_NAME=molii-production
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://molii.co
```

Development 结构：

```dotenv
SQL_DSN=postgresql://molii_dev_app:URL_ENCODED_PASSWORD@POSTGRES_HOST:5432/molii_dev?sslmode=require
REDIS_CONN_STRING=redis://REDIS_USER:URL_ENCODED_PASSWORD@DEV_MOLIICO_REDIS_HOST:6379/0
SESSION_SECRET=DEVELOPMENT_RANDOM_HEX
CRYPTO_SECRET=DEVELOPMENT_RANDOM_HEX
TZ=Asia/Shanghai
ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
NODE_NAME=molii-development
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://dev.molii.co
```

URI 中的用户名或密码若包含 `@`、`:`、`/`、`?`、`#`、`%`，必须进行 URL 编码。云 PostgreSQL/Redis 的网络白名单只放行服务器出口 IP，并按云厂商要求启用 TLS。

完成后验证文件非空且权限正确；命令只显示键名，不显示值：

```bash
sudo -u molii-deploy awk -F= 'NF >= 2 && length($2) > 0 {print $1 "=SET"}' /opt/molii/production/.env.runtime
sudo -u molii-deploy awk -F= 'NF >= 2 && length($2) > 0 {print $1 "=SET"}' /opt/molii/development/.env.runtime
stat -c '%a %U:%G %n' /opt/molii/production/.env.runtime /opt/molii/development/.env.runtime
```

两个文件应显示权限 `600`，并归 `molii-deploy:molii-deploy` 所有。

## 四、配置 1Panel 反向代理

先确认 1Panel OpenResty 使用宿主机网络模式：

```bash
docker inspect 1Panel-openresty --format '{{.HostConfig.NetworkMode}}'
```

不同 1Panel 版本的容器名可能不同，可先用 `docker ps --format '{{.Names}}' | grep -i openresty` 查找。输出必须是 `host`，此时 OpenResty 中的 `127.0.0.1` 才是宿主机回环地址；1Panel 默认 OpenResty 配置采用该模式。如果输出不是 `host`，不要直接继续创建这两个代理，应先恢复 1Panel 默认 OpenResty 网络配置。

在 1Panel 的“网站 → 创建网站 → 反向代理”中创建：

1. `molii.co` 代理到 `http://127.0.0.1:3000`。
2. `dev.molii.co` 代理到 `http://127.0.0.1:3010`。
3. 两个站点都申请 ACME 证书、开启自动续签并强制 HTTP 跳转 HTTPS。
4. 保留流式响应所需的代理缓冲/超时配置；不要给应用端口添加公网安全组规则。

首次部署前代理返回 `502` 属于正常现象，容器启动后才会通过健康检查。

## 五、配置 GitHub Secrets 与 Environments

在 GitHub 仓库进入 `Settings → Secrets and variables → Actions`，添加：

| Secret | 内容 |
| --- | --- |
| `DEPLOY_SSH_HOST` | 服务器域名或固定公网 IP |
| `DEPLOY_SSH_PORT` | SSH 端口，例如 `22` |
| `DEPLOY_SSH_USER` | `molii-deploy` |
| `DEPLOY_SSH_PRIVATE_KEY` | `molii-deploy-github` 私钥的完整多行内容 |
| `DEPLOY_SSH_KNOWN_HOSTS` | 已核对指纹的 `ssh-keyscan` 完整输出 |
| `TELEGRAM_BOT_TOKEN` | BotFather 创建的机器人令牌 |
| `TELEGRAM_CHAT_ID` | 接收通知的个人、群组或频道 chat ID |

数据库、Redis、`SESSION_SECRET`、`CRYPTO_SECRET` 不应创建为 GitHub Secrets。

在 `Settings → Environments` 创建：

- `development`
- `production`

建议给 `production` 配置 required reviewer，Development 保持自动部署。工作流通过仓库自带的 `GITHUB_TOKEN` 发布和拉取同仓库 GHCR 包；不需要长期 GHCR PAT。首次构建后确认 GHCR package 已关联当前仓库并继承 Actions 访问权限。

## 六、配置 Telegram

1. 在 Telegram 联系 `@BotFather`，创建机器人并取得 token。
2. 将机器人加入目标群组或频道；频道需要授予发消息权限。
3. 给机器人发送一条测试消息，再通过 Telegram Bot API 的 `getUpdates` 查看 `chat.id`。
4. 将 token 和 chat ID 存入上述 GitHub Secrets。

不要在 shell 历史中直接写 token。测试时先使用静默输入：

```bash
read -r -s TELEGRAM_BOT_TOKEN
export TELEGRAM_BOT_TOKEN
read -r TELEGRAM_CHAT_ID
export TELEGRAM_CHAT_ID
curl --silent --show-error --fail \
  --request POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
  --data-urlencode "chat_id=$TELEGRAM_CHAT_ID" \
  --data-urlencode 'text=Molii CI/CD Telegram test'
unset TELEGRAM_BOT_TOKEN TELEGRAM_CHAT_ID
```

## 七、第一次发布

1. 先合并 CI/CD 变更到 `develop`。
2. `develop` push 触发 Development 构建与部署。
3. 在 Actions 中确认 Verify、Build、Deploy、Notify 全部成功。
4. 验证：

   ```bash
   curl --fail --silent --show-error https://dev.molii.co/api/status
   ```

5. 登录 Development，验证数据库迁移、Redis、登录会话与核心 API。
6. Development 稳定后，将同一变更合并到 `main`；Production 环境审批后部署到 `molii.co`。

每次部署使用 `ghcr.io/2hot4you/new-api@sha256:...` digest。`development`、`production` 标签仅用于查看，服务器不依赖可变标签。

## 八、排查与恢复

查看环境状态：

```bash
cd /opt/molii/development
docker compose --env-file .deploy.env ps
docker logs --tail 100 molii-development
curl --fail --silent --show-error https://dev.molii.co/api/status
```

Production 将目录和容器名替换为 `/opt/molii/production`、`molii-production`。

部署脚本会在新容器或公网健康检查失败时自动恢复部署前的镜像，并让 Actions 保持失败。如果是第一次部署，没有旧镜像可恢复，失败容器会保留供查看日志。

手工重新应用 `.deploy.env` 中已经记录的镜像：

```bash
cd /opt/molii/development
docker compose --env-file .deploy.env up -d --remove-orphans
```

不要运行 `docker system prune --volumes`，否则可能破坏其他 1Panel 应用。不要删除 `data/`、`logs/` 或 `.env.runtime`。

## 九、持续健康监控

部署脚本和 Actions 只负责发布期健康检查及 Telegram 结果通知。持续监控建议在 1Panel 安装 Uptime Kuma：

- Monitor 1：`https://molii.co/api/status`，断言 HTTP 200 且正文包含 `"success":true`。
- Monitor 2：`https://dev.molii.co/api/status`，使用相同断言。
- 检查间隔建议 60 秒，失败重试 3 次后告警。
- 配置 Telegram 故障和恢复通知，避免使用无状态 cron 每分钟重复刷屏。
