# Review

- 确认旧常驻进程 PID 为 36926，运行的是改名前二进制。
- 使用当前提交重新构建本机 arm64 后端，并原子替换 LaunchAgent 使用的二进制。
- 通过 `launchctl kickstart -k` 重启 `com.molii.new-api`。
- 新进程 PID 为 63902，父进程为 launchd，继续使用 KeepAlive 常驻。
- `127.0.0.1:3000/api/status` 返回 HTTP 200 且 `success: true`。
- 启动日志无致命错误；仅有既有 `TRUSTED_PROXIES` 配置警告。
