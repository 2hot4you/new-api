# model.claudeye.com 独立生产环境 CI/CD

状态：待后续实施。

## 已确认目标

- 新增 `model.claudeye.com` 生产环境，部署在 BandwagonHost。
- 从 `main` 分支独立部署，不影响现有 `molii.co` 与 `aigc.ixiaozu.cn`。
- 使用该服务器自建的 PostgreSQL 与 Redis，并与其他环境完全隔离。
- 独立数据库、Redis、用户体系、渠道、密钥、品牌配置和运行时密钥。
- 延续现有多环境 CI/CD 的健康检查、部署锁、回滚和网络重试策略。

## 实施前需确认

- 服务器系统、CPU 架构、磁盘与备份空间。
- SSH 用户、端口、Host Key 和 GitHub Environment Secrets。
- PostgreSQL/Redis 的容器卷、TLS、备份、恢复和升级方案。
- 域名 DNS、证书、OpenResty/Nginx 反代和应用监听端口。
- GitHub Environment 名称、站点品牌变量与健康检查 URL。

本记录仅保留需求；当前 Seedance 可观测性任务不实施此环境。
