# 诊断结果

- 根因：`SESSION_COOKIE_SECURE=true` 时缺少 `SESSION_COOKIE_TRUSTED_URL`，进程按安全策略以退出码 1 停止。
- 修复：生产环境配置 `SESSION_COOKIE_TRUSTED_URL=https://aigc.claudeye.com` 后强制重建应用容器。
- 验收：New API、PostgreSQL 和 Redis 均为 healthy，迁移服务退出码为 0。
- 线上验证：`https://aigc.claudeye.com/api/status` 返回 HTTP 200。
- 部署端口：宿主机 `127.0.0.1:3030` 转发到容器 `3000`。
- 数据卷未删除，数据库未重建。
