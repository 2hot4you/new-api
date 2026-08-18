# new-api 双环境 CI/CD 设计

## 目标

当代码推送到 `develop` 或 `main` 时，GitHub Actions 自动验证代码、构建 amd64 Docker 镜像、推送到 GHCR，并通过 SSH 将指定镜像部署到同一台 Ubuntu 24.04 + 1Panel 服务器。部署必须经过容器健康检查和公网 `/api/status` 检查，失败时自动恢复上一镜像，并通过 Telegram 报告结果。

## 环境映射

| Git 分支 | 部署环境 | 公网地址 | 主机监听 | 容器端口 | 服务器目录 |
| --- | --- | --- | --- | --- | --- |
| `main` | production | `https://molii.co` | `127.0.0.1:3000` | `3000` | `/opt/molii/production` |
| `develop` | development | `https://dev.molii.co` | `127.0.0.1:3010` | `3000` | `/opt/molii/development` |

1Panel/OpenResty 只负责 TLS 终止和反向代理。应用端口不对公网开放。

## 发布数据流

1. GitHub Actions 对后端、独立 `relaykit` 模块、前端类型/生产构建和部署契约运行验证。完整前端测试继续由现有 PR CI 负责，不在发布工作流中重复执行。
2. Actions 使用仓库 `Dockerfile` 构建 `linux/amd64` 镜像并推送到 `ghcr.io/2hot4you/new-api`。
3. 每次部署使用镜像 digest，而不是可变标签，确保部署内容可追溯。
4. Actions 通过固定 `known_hosts` 和专用 SSH 私钥连接服务器，将 Compose 文件和部署脚本复制到目标环境目录。
5. 服务器从本地 `.env.runtime` 读取数据库、Redis 和会话密钥；这些秘密不进入 GitHub 或镜像。
6. 部署脚本记录当前镜像、拉取新镜像、更新 Compose，并等待容器健康。
7. 容器健康后，脚本验证环境对应的公网 `/api/status` 返回 `success: true`。
8. 任一检查失败时恢复原镜像；Actions 任务保持失败状态并发送 Telegram 通知。

## 凭据边界

GitHub Repository Secrets 只保存：

- `DEPLOY_SSH_HOST`
- `DEPLOY_SSH_PORT`
- `DEPLOY_SSH_USER`
- `DEPLOY_SSH_PRIVATE_KEY`
- `DEPLOY_SSH_KNOWN_HOSTS`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`

数据库、Redis、`SESSION_SECRET` 和 `CRYPTO_SECRET` 只保存在服务器的 `.env.runtime` 中，权限为 `0600`。正式和测试环境各自使用独立 PostgreSQL 数据库/角色和独立 Redis 实例。

## 服务器布局

每个环境目录包含：

- `docker-compose.yml`：由 Actions 发布，可覆盖。
- `deploy.sh`：由 Actions 发布，可覆盖。
- `.env.runtime`：管理员首次创建，Actions 不覆盖。
- `.deploy.env`：部署脚本维护，只含镜像引用、端口和容器名。
- `data/`、`logs/`：环境独立的持久化目录。

部署 SSH 用户必须能够运行 Docker，并且只需拥有 `/opt/molii/production` 与 `/opt/molii/development`。不要求 GitHub Actions 调用 1Panel API。

## 回滚与并发

Actions 按环境设置 concurrency，同一环境只允许一个发布；正在执行的发布不会被新提交强制取消，新发布进入等待。部署脚本使用 `flock` 防止服务器侧并发。回滚仅切换应用镜像；数据库迁移由 new-api 启动过程管理，因此发布前必须保持迁移向后兼容。

首次部署不存在旧镜像时无法自动回滚，失败容器会保留供排查。后续部署会恢复到部署前实际运行的镜像。

## 通知与持续监控

每次部署无论成功、失败或取消都发送 Telegram，内容包含环境、分支、提交、操作者和 Actions 链接。部署期健康检查由脚本负责。

长期运行监控建议在 1Panel 安装 Uptime Kuma，分别监控两个 `/api/status` 地址并配置 Telegram 的故障与恢复通知；定时监控不放进 GitHub Actions，以避免故障期间重复刷屏和依赖 GitHub 调度延迟。

## 验证标准

- 部署脚本测试覆盖环境映射、必需环境变量、成功部署和失败回滚。
- Shell 脚本通过 `bash -n`。
- Compose 配置在 production/development 两套变量下通过 `docker compose config --quiet`。
- GitHub Actions YAML 通过 actionlint。
- `go test ./...`、`cd relaykit && GOWORK=off go test ./...` 和 `bun run build:check` 通过。
- 当前 `develop` 的完整 `bun test` 基线为 138 通过、69 失败、61 个加载错误；本任务不修改这些无关测试，结果记录在任务审查中。
