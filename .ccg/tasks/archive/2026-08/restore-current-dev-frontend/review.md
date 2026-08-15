# Review

## Root cause

- 3000 重启后被 `com.molii.new-api` LaunchAgent 接管。
- 该 LaunchAgent 原先运行 2026-08-07 编译的旧二进制，内嵌旧版 `web/dist`，因此当前仓库首页改动没有显示。
- 当前仓库源码和提交均未丢失。

## Resolution

- 将 `com.molii.new-api` 的启动脚本切换为当前仓库 `go run .`，并设置 `FRONTEND_DEV_SERVER_URL=http://127.0.0.1:3001`。
- 新增 `com.molii.new-api-web` LaunchAgent，常驻运行 Rsbuild 开发服务并仅监听 `127.0.0.1:3001`。
- 3000 继续作为唯一浏览器入口；3001 仅承载内部热更新流量。
- 未删除旧二进制，未执行前端生产构建。

## Verification

- `127.0.0.1:3000/api/status`：通过。
- 3000 页面响应来自 Rsbuild 开发服务：通过。
- 浏览器渲染包含“连接每一种 AI 能力。用 Molii 开始创造。”及新版 Molii 能力、模型生态与 Footer：通过。
- `com.molii.new-api` 与 `com.molii.new-api-web` 均由 LaunchAgent 常驻运行。
