# 云数据库与持续部署实施建议

## 职责划分

- 云控制台负责创建 PostgreSQL、Redis、网络白名单、TLS、备份与监控。
- 一次性迁移流程负责导出、恢复、数据核验与连接切换，不放进每次 Git push 的工作流。
- GitHub Actions 负责测试、构建不可变 Docker 镜像并通知服务器更新应用。
- 服务器保存生产连接串和业务密钥，GitHub 不保存数据库密码。

## 推荐顺序

1. 创建相互隔离的 staging 与 production 数据库资源。
2. 修改生产 Compose，使用 `SQL_DSN` 和 `REDIS_CONN_STRING`，去掉对本地数据库容器的运行依赖。
3. 备份并迁移当前 PostgreSQL，验证关键表与业务接口。
4. 切换正式服务连接串，保留原卷用于回滚。
5. 创建 staging 分支和测试部署环境。
6. 配置 staging push 自动部署。
7. 配置 main push 自动构建，并由 production Environment 控制正式部署。

## 当前缺口

- `deploy/docker-compose.yml` 将 PostgreSQL 和 Redis 主机写死为 Compose 服务名。
- `migrate` 服务将 PostgreSQL 主机写死为 `postgres`。
- `docker-image-branch.yml` 仅手动触发并将镜像名称写成 `calciumion/new-api`。
- 尚无 staging/production 自动部署工作流、部署并发锁、外部健康检查和回滚步骤。
