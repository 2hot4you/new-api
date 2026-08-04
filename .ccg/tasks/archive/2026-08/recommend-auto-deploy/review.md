# 自动部署方案审查

## 结论

推荐采用 GitHub Actions 构建并推送 Docker Hub 镜像，再通过 SSH 在生产服务器执行部署。生产部署仅重建 `new-api` 服务，不执行 `docker compose down`，因此 PostgreSQL 与 Redis 保持运行。

## 当前仓库现状

- `deploy/docker-compose.yml` 已支持通过 `NEW_API_IMAGE` 指定镜像，且 `new-api` 配置了健康检查与 `restart: unless-stopped`。
- `migrate` 服务负责执行数据库迁移，生产部署需在应用更新前显式运行。
- 现有分支镜像工作流仅支持手动触发，并将镜像名称硬编码为 `calciumion/new-api`，与当前使用的 `pangmaom1/new-api` 不一致。
- 当前没有 push 后自动部署生产服务器的工作流。

## 实施边界

- 仅在指定生产分支 push 后部署，功能分支不直接触发生产更新。
- 镜像使用不可变 `sha-<commit>` 标签；可额外维护一个可读别名，但生产实际部署使用 SHA 标签。
- 服务器保留 `deploy/.env`，不把数据库、Redis、Cookie 等生产密钥写入 GitHub 仓库或镜像。
- 部署任务设置 concurrency，避免多个 push 并发更新同一台服务器。
- SSH 校验服务器 host key，不关闭主机身份验证。
- 失败时保留并可重新部署上一版本 SHA 镜像。

## 未实施原因

本任务为方案咨询。自动部署会改动 GitHub Actions 并要求配置生产服务器 SSH 权限，需用户确认触发部署的生产分支后再实施。
