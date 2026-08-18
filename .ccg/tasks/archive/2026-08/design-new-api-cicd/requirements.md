# CI/CD requirements

- `main` 自动部署到 `molii.co`，反向代理目标 `127.0.0.1:3000`。
- `develop` 自动部署到 `dev.molii.co`，反向代理目标 `127.0.0.1:3010`。
- GitHub Actions 构建 new-api Docker 镜像并推送 GHCR。
- 服务器使用 Ubuntu 24.04、1Panel、Docker 与 OpenResty。
- PostgreSQL 使用独立数据库/应用角色，Redis 使用 `moliico` 与 `dev-moliico` 两个实例。
- 运行时数据凭据只存放在服务器，GitHub 不保存数据库或 Redis 密码。
- 部署后检查容器和 `/api/status`，失败恢复上一镜像。
- 部署结果通过 Telegram 通知。
