# 需求

## 目标

- `develop` 分支把 Docusaurus 静态站部署到 `https://dev.molii.co/docs/`。
- `main` 分支把 Docusaurus 静态站部署到 `https://molii.co/docs/`。
- `/docs` 与 `/docs/` 由 OpenResty 重定向到 `/docs/quick-start`。
- 文档使用 Docusaurus 的 `/docs/` `baseUrl` 构建，不启动服务器端文档进程，也不占用 3100/3001/3011 端口。
- GitHub Actions 复用现有部署 SSH 与 Telegram Secrets，不新增应用、数据库或对象存储凭据。
- 文档部署与 new-api 容器部署职责分离，失败不得影响当前运行中的 new-api。

## 现有服务器约束

- 生产静态根目录：`/opt/1panel/www/sites/molii.co/index`。
- 开发静态根目录：`/opt/1panel/www/sites/dev.molii.co/index`。
- OpenResty 容器内对应 `/www/sites/<host>/index`。
- `/opt/molii/production` 与 `/opt/molii/development` 包含运行时秘密和日志，不作为公开静态根目录。
- 用户已保存 `/docs` 静态路由配置；实施不登录服务器修改 OpenResty 或生产配置。

## 安全与运维边界

- 构建产物只能包含公开配置：`DOCS_ENV`、`DOCS_SITE_URL`、`DOCS_BASE_URL`、`DOCS_API_BASE_URL`。
- 不读取、输出或复制 `.env.runtime`、数据库、Redis、API Token 或其他 Secret。
- 部署前保存上一份文档快照；发布后检查重定向和快速开始页，失败时恢复快照。
- production 与 development 使用独立并发锁和目标目录。
- 不修改 `main`，不直接操作生产环境；生产部署仅由后续 `main` push 触发。

## 验收

- `/docs` 返回到 `/docs/quick-start` 的永久重定向。
- `/docs/quick-start`、静态资源、站内导航和搜索资源均在 `/docs/` 下正常访问。
- Development 构建带 `noindex`，Production 允许索引。
- 文档变更可独立触发文档工作流，不必重建 new-api Docker 镜像。
- 部署成功或失败均发送不含敏感信息的 Telegram 通知。
