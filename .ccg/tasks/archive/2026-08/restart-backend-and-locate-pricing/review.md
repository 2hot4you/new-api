# Review

- 确认原进程运行的是 2026-08-03 构建的旧二进制。
- 从当前 `feat/molii-auth` 源码重新构建并替换 launchd 管理的本地二进制，旧版本已保留以便回滚。
- `com.molii.new-api` 保持 `KeepAlive` 和 `RunAtLoad`，新进程已监听 TCP 3000。
- `GET http://127.0.0.1:3000/api/status` 返回成功。
- 价格配置入口确认为 `/system-settings/billing/molii-aigc-video-pricing`，仅超级管理员可访问。
- 本任务未修改业务源码或配置密钥。
