# Molii 双环境 CI/CD 操作手册

本手册部署当前仓库的两个环境：

| 环境 | 分支 | 域名 | 1Panel 反向代理 | 服务器目录 |
| --- | --- | --- | --- | --- |
| Production | `main` | `molii.co` | `127.0.0.1:3000` | `/opt/molii/production` |
| Development | `develop` | `dev.molii.co` | `127.0.0.1:3010` | `/opt/molii/development` |

运行时秘密只存放在服务器。GitHub Actions 构建镜像、推送到 GHCR，并通过 SSH 调用仓库内的部署脚本。

## 开始前：严格按顺序执行

不要按照聊天消息里的临时编号跳步，以本手册的章节编号为准。每个阶段的“完成检查点”通过后才能继续：

| 顺序 | 操作位置 | 内容 |
| --- | --- | --- |
| 一 | Ubuntu 服务器管理员终端 | 检查依赖，创建部署用户和目录 |
| 二 | Mac、服务器管理员终端 | 生成 SSH 密钥、安装公钥、核对主机指纹并测试登录 |
| 三 | Ubuntu 服务器管理员终端 | 创建 Production 和 Development 运行时环境文件 |
| 四 | 服务器管理员终端、1Panel 网页 | 配置两个域名的 HTTPS 反向代理 |
| 五 | Telegram、Mac | 创建机器人、取得 chat ID 并测试通知 |
| 六 | GitHub 网页、Mac | 创建 GitHub Environments 和 Repository Secrets |
| 七 | Git 和 GitHub Actions | 首次发布 Development，验证后再发布 Production |

操作位置含义：

- **Mac**：你的可信 macOS 电脑，不是服务器。
- **服务器管理员终端**：使用 `root` 或已有 sudo 权限的管理员账号；可以通过 SSH 或 1Panel 终端进入。
- **`molii-deploy` 终端**：只用于验证部署账号权限，不给该账号设置密码或 sudo 权限。
- **1Panel 网页**：服务器的 1Panel 管理界面。
- **GitHub 网页**：`new-api` 仓库的 Settings 页面。
- **Telegram**：Telegram 客户端和 Bot API。

在第七节之前，不要把 CI/CD 变更合并或推送到 `develop`/`main`，否则工作流会在服务器尚未准备好时开始部署。

## 一、服务器一次性准备

### 1.1 检查服务器依赖

**操作位置：Ubuntu 服务器管理员终端**

前置条件为 Ubuntu 24.04、1Panel、Docker、Docker Compose v2 和 OpenResty。执行：

```bash
sudo docker version
sudo docker compose version
command -v curl
command -v flock
```

四项都必须成功。`flock` 通常由 Ubuntu 的 `util-linux` 包提供。

### 1.2 创建部署用户和目录

**操作位置：Ubuntu 服务器管理员终端**

先检查用户是否已经存在：

```bash
id molii-deploy
```

如果提示用户不存在，创建它：

```bash
sudo adduser --disabled-password --gecos '' molii-deploy
```

无论用户是新建还是已经存在，都执行：

```bash
sudo usermod -aG docker molii-deploy
sudo install -d -m 0750 -o molii-deploy -g molii-deploy /opt/molii/production
sudo install -d -m 0750 -o molii-deploy -g molii-deploy /opt/molii/development
sudo install -d -m 0750 -o molii-deploy -g molii-deploy /opt/molii/production/data /opt/molii/production/logs
sudo install -d -m 0750 -o molii-deploy -g molii-deploy /opt/molii/development/data /opt/molii/development/logs
```

`molii-deploy` 是无密码部署账号。不要给它设置 sudo 密码；需要管理员权限的操作继续使用服务器管理员账号。

### 1.3 完成检查点

**操作位置：Ubuntu 服务器管理员终端**

```bash
sudo -iu molii-deploy sh -lc 'id && docker info >/dev/null && docker compose version'
stat -c '%a %U:%G %n' /opt/molii/production /opt/molii/development
```

结果必须满足：

- `id` 输出包含 `docker` 用户组。
- `docker info` 没有 permission denied。
- `docker compose version` 成功。
- 两个目录归 `molii-deploy:molii-deploy` 所有。

如果失败，停在本节排查，不要把 Docker socket 改成全局可写。

## 二、配置部署 SSH 密钥

### 2.1 在 Mac 生成专用密钥

**操作位置：Mac**

先检查是否已经生成过，避免覆盖现有私钥：

```bash
ls -l ~/.ssh/molii-deploy-github ~/.ssh/molii-deploy-github.pub
```

如果两个文件已经存在，跳过生成命令。如果不存在，执行：

```bash
mkdir -p ~/.ssh
chmod 700 ~/.ssh
ssh-keygen -t ed25519 -C 'github-actions-molii-deploy' -f ~/.ssh/molii-deploy-github
```

设置口令时直接按两次 Enter，不设置交互式口令。然后执行：

```bash
chmod 600 ~/.ssh/molii-deploy-github
chmod 644 ~/.ssh/molii-deploy-github.pub
```

- `~/.ssh/molii-deploy-github` 是私钥，只保存在 Mac 和后续的 GitHub Secret 中。
- `~/.ssh/molii-deploy-github.pub` 是公钥，只把这一份安装到服务器。
- 不要把任何一个文件放进项目仓库；绝对不要把私钥上传服务器或发送到聊天。

### 2.2 将公钥安装到服务器

先在 **Mac** 复制公钥：

```bash
pbcopy < ~/.ssh/molii-deploy-github.pub
```

然后切换到 **Ubuntu 服务器管理员终端**：

```bash
sudo install -d -m 0700 -o molii-deploy -g molii-deploy /home/molii-deploy/.ssh
sudo touch /home/molii-deploy/.ssh/authorized_keys
sudo chown molii-deploy:molii-deploy /home/molii-deploy/.ssh/authorized_keys
sudo chmod 0600 /home/molii-deploy/.ssh/authorized_keys
sudo nano /home/molii-deploy/.ssh/authorized_keys
```

把剪贴板中的公钥完整粘贴为一行，保存并退出。不要粘贴私钥。每个公钥占一行；如果同一个公钥已经存在，不要重复添加。

如果当前终端已经是 `molii-deploy`，不要使用 sudo，改为：

```bash
install -d -m 0700 ~/.ssh
touch ~/.ssh/authorized_keys
chmod 0600 ~/.ssh/authorized_keys
nano ~/.ssh/authorized_keys
```

### 2.3 核对服务器主机指纹

先在 **Ubuntu 服务器管理员终端** 查看服务器真实指纹：

```bash
sudo ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

记录完整的 `SHA256:...`。然后在 **Mac** 执行：

```bash
ssh-keyscan -p 22 -t ed25519 43.132.166.73 > ~/.ssh/molii-deploy-known-hosts
chmod 600 ~/.ssh/molii-deploy-known-hosts
ssh-keygen -lf ~/.ssh/molii-deploy-known-hosts
```

Mac 和服务器输出的 `SHA256:...` 必须完全一致。这里的主机和端口必须与后续 GitHub Secrets 完全一致。指纹不一致时立即停止。

### 2.4 从 Mac 测试部署账号登录

**操作位置：Mac**

```bash
ssh \
  -p 22 \
  -i ~/.ssh/molii-deploy-github \
  -o IdentitiesOnly=yes \
  -o UserKnownHostsFile=~/.ssh/molii-deploy-known-hosts \
  -o StrictHostKeyChecking=yes \
  molii-deploy@43.132.166.73 \
  'whoami && docker info >/dev/null && docker compose version'
```

### 2.5 完成检查点

必须满足：

- 登录过程没有询问 `molii-deploy` 密码。
- `whoami` 输出 `molii-deploy`。
- Docker 和 Docker Compose 检查成功。
- `~/.ssh/molii-deploy-known-hosts` 已保留在 Mac，供第六节使用。

只有全部满足，才能进入第三节。暂时不要配置 GitHub。

## 三、创建服务器运行时环境文件

数据库、Redis 和应用密钥只保存在服务器的 `.env.runtime`，不要放进 GitHub Secrets。

### 3.1 创建两个受保护的环境文件

**操作位置：Ubuntu 服务器管理员终端**

```bash
sudo install -m 0600 -o molii-deploy -g molii-deploy /dev/null /opt/molii/production/.env.runtime
sudo install -m 0600 -o molii-deploy -g molii-deploy /dev/null /opt/molii/development/.env.runtime
```

注意：这两条命令会清空同名文件。如果文件中已经填写过真实配置，不要再次执行，直接编辑并检查现有文件。

### 3.2 生成四个互不相同的应用密钥

**操作位置：Ubuntu 服务器管理员终端**

逐条执行并立即保存到密码管理器：

```bash
openssl rand -hex 32
openssl rand -hex 32
openssl rand -hex 32
openssl rand -hex 32
```

按顺序分别用于 Production `SESSION_SECRET`、Production `CRYPTO_SECRET`、Development `SESSION_SECRET`、Development `CRYPTO_SECRET`。不要把输出发送到聊天或提交到 Git。

### 3.3 填写 Production

**操作位置：Ubuntu 服务器管理员终端**

```bash
sudo -u molii-deploy nano /opt/molii/production/.env.runtime
```

填写并替换所有占位值：

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

### 3.4 填写 Development

**操作位置：Ubuntu 服务器管理员终端**

```bash
sudo -u molii-deploy nano /opt/molii/development/.env.runtime
```

填写并替换所有占位值：

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

要求：

- Production 使用 `molii_prod_app`/`molii_prod` 和正式 Redis 实例 `moliico`。
- Development 使用 `molii_dev_app`/`molii_dev` 和测试 Redis 实例 `dev-moliico`。
- URI 中的用户名或密码包含 `@`、`:`、`/`、`?`、`#`、`%` 时必须 URL 编码。
- Redis 若要求 TLS，使用云厂商给出的 `rediss://` 地址和端口。
- PostgreSQL 和 Redis 网络白名单只放行服务器出口 IP，并按云厂商要求启用 TLS。

### 3.5 完成检查点

**操作位置：Ubuntu 服务器管理员终端**

以下命令只显示键名和设置状态，不显示秘密值：

```bash
sudo -u molii-deploy awk -F= 'NF >= 2 && length($2) > 0 {print $1 "=SET"}' /opt/molii/production/.env.runtime
sudo -u molii-deploy awk -F= 'NF >= 2 && length($2) > 0 {print $1 "=SET"}' /opt/molii/development/.env.runtime
stat -c '%a %U:%G %n' /opt/molii/production/.env.runtime /opt/molii/development/.env.runtime
```

两个文件都必须满足：

- 四个必填项显示 `SET`。
- 权限为 `600`，所有者为 `molii-deploy:molii-deploy`。
- 没有遗留 `URL_ENCODED_PASSWORD`、`POSTGRES_HOST`、`RANDOM_HEX` 等占位内容。

## 四、配置 1Panel 反向代理

### 4.1 确认 OpenResty 网络模式

**操作位置：Ubuntu 服务器管理员终端**

```bash
sudo docker ps --format '{{.Names}}' | grep -i openresty
```

把下面的 `OPENRESTY_CONTAINER_NAME` 换成上一步输出：

```bash
sudo docker inspect OPENRESTY_CONTAINER_NAME --format '{{.HostConfig.NetworkMode}}'
```

输出必须是 `host`，此时 OpenResty 中的 `127.0.0.1` 才指向服务器宿主机。如果不是 `host`，停在这里恢复 1Panel 默认 OpenResty 网络配置。

### 4.2 创建两个网站

**操作位置：1Panel 网页**

进入“网站 → 创建网站 → 反向代理”，分别创建：

1. `molii.co` → `http://127.0.0.1:3000`
2. `dev.molii.co` → `http://127.0.0.1:3010`

两个网站都要：

- 申请 ACME TLS 证书并开启自动续签。
- 强制 HTTP 跳转 HTTPS。
- 保留流式响应需要的代理超时设置。
- 不在安全组或系统防火墙中开放 `3000`、`3010` 公网访问。

### 4.3 完成检查点

**操作位置：Mac**

```bash
curl -sS -o /dev/null -w '%{http_code}\n' https://molii.co/
curl -sS -o /dev/null -w '%{http_code}\n' https://dev.molii.co/
```

首次部署前应用容器尚未启动，返回 `502` 通常正常。这里主要确认域名能访问到 1Panel、HTTPS 证书有效且没有 TLS/DNS 错误。

## 五、配置 Telegram

### 5.1 创建机器人并取得 chat ID

**操作位置：Telegram**

1. 联系 `@BotFather` 创建机器人并取得 bot token。
2. 将机器人加入目标个人会话、群组或频道；频道需要授予发消息权限。
3. 向机器人或包含机器人的群组发送一条测试消息。

然后在 **Mac** 上静默输入 token 并查询更新：

```bash
read -r -s TELEGRAM_BOT_TOKEN
export TELEGRAM_BOT_TOKEN
printf '\n'
curl --silent --show-error --fail \
  "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getUpdates" \
  | python3 -m json.tool
```

在输出中找到目标会话的 `chat.id`。群组 chat ID 通常是负数。

### 5.2 测试通知

**操作位置：Mac，继续使用上一步终端**

```bash
read -r TELEGRAM_CHAT_ID
export TELEGRAM_CHAT_ID
curl --silent --show-error --fail \
  --request POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
  --data-urlencode "chat_id=$TELEGRAM_CHAT_ID" \
  --data-urlencode 'text=Molii CI/CD Telegram test'
unset TELEGRAM_BOT_TOKEN TELEGRAM_CHAT_ID
```

收到测试消息后，把 bot token 和 chat ID 保存到密码管理器，进入下一节。不要把它们发送到聊天或写入仓库文件。

## 六、配置 GitHub Secrets 与 Environments

只有第一至第五节全部通过后，才执行本节。

### 6.1 创建 Environments

**操作位置：GitHub 网页**

进入仓库 `Settings → Environments`，分别创建：

- `development`
- `production`

名称必须完全一致且使用小写。建议给 `production` 配置 required reviewer，Development 保持自动部署。

### 6.2 创建 Repository Secrets

**操作位置：GitHub 网页和 Mac**

进入仓库 `Settings → Secrets and variables → Actions → Repository secrets`，逐个创建：

| Secret | 内容 |
| --- | --- |
| `DEPLOY_SSH_HOST` | `43.132.166.73` |
| `DEPLOY_SSH_PORT` | `22` |
| `DEPLOY_SSH_USER` | `molii-deploy` |
| `DEPLOY_SSH_PRIVATE_KEY` | Mac 上 `~/.ssh/molii-deploy-github` 的完整多行内容 |
| `DEPLOY_SSH_KNOWN_HOSTS` | Mac 上 `~/.ssh/molii-deploy-known-hosts` 的完整内容 |
| `TELEGRAM_BOT_TOKEN` | 第五节验证过的机器人 token |
| `TELEGRAM_CHAT_ID` | 第五节验证过的 chat ID |

在 **Mac** 上复制私钥：

```bash
pbcopy < ~/.ssh/molii-deploy-github
```

粘贴到 `DEPLOY_SSH_PRIVATE_KEY`，内容必须包含完整开头和结尾：

```text
-----BEGIN OPENSSH PRIVATE KEY-----
...
-----END OPENSSH PRIVATE KEY-----
```

复制已验证的主机记录：

```bash
pbcopy < ~/.ssh/molii-deploy-known-hosts
```

粘贴到 `DEPLOY_SSH_KNOWN_HOSTS`，内容类似：

```text
43.132.166.73 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...
```

数据库、Redis、`SESSION_SECRET`、`CRYPTO_SECRET` 不应创建为 GitHub Secrets，它们只存在于服务器 `.env.runtime`。

### 6.3 完成检查点

确认：

- Environments 中存在 `development` 和 `production`。
- Repository secrets 中能看到上述七个名称；GitHub 不会再次显示 Secret 的值。
- `DEPLOY_SSH_HOST` 与第二节 `ssh-keyscan` 使用同一个主机值。
- 此时仍未把 CI/CD 变更推送到 `develop` 或 `main`。

工作流使用仓库自带的 `GITHUB_TOKEN` 发布和拉取同仓库 GHCR 镜像，不需要长期 GHCR PAT。

## 七、第一次发布

### 7.1 发布 Development

第一至第六节全部完成后：

1. 将 CI/CD 变更合并到 `develop` 并推送到 GitHub。
2. `develop` push 自动触发 Development 构建与部署。
3. 在 GitHub Actions 中确认 Resolve、Verify、Build、Deploy、Notify 的结果。
4. 验证：

   ```bash
   curl --fail --silent --show-error https://dev.molii.co/api/status
   ```

5. 登录 Development，验证数据库连接与迁移、Redis、登录会话和核心 API。
6. 确认 Telegram 收到 Development 部署结果。

不要在功能分支上手动运行 `workflow_dispatch`；部署工作流只接受 `develop` 和 `main`。

### 7.2 发布 Production

Development 稳定后：

1. 将相同变更从 `develop` 合并到 `main`。
2. 推送 `main`，在 GitHub `production` Environment 中审批部署。
3. 验证：

   ```bash
   curl --fail --silent --show-error https://molii.co/api/status
   ```

4. 验证 Production 数据库、正式 Redis、登录会话和核心 API。
5. 确认 Telegram 收到 Production 部署结果。

每次部署使用 `ghcr.io/2hot4you/new-api@sha256:...` digest。`development`、`production` 标签仅用于查看，服务器不依赖可变标签。

## 八、排查与恢复

查看 Development 状态：

```bash
cd /opt/molii/development
docker compose --env-file .deploy.env ps
docker logs --tail 100 molii-development
curl --fail --silent --show-error https://dev.molii.co/api/status
```

Production 将目录和容器名替换为 `/opt/molii/production`、`molii-production`。

部署脚本会在新容器或公网健康检查失败时自动恢复部署前的镜像，并让 Actions 保持失败。如果是第一次部署，没有旧镜像可恢复，失败容器会保留供查看日志。

手工重新应用 `.deploy.env` 中已记录的镜像：

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
