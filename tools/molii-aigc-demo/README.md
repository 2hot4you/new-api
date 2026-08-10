# Molii AIGC API Test Lab

独立的 Molii AIGC API 测试台，用于验证不同 New API 环境中的 Seedance 与 Grok Imagine 生图、生视频和素材接口。

它不依赖 New API 自带前端，也不会把 Base URL、API Key 或历史记录保存到浏览器。环境、任务与完整 HTTP 交换记录保存在本地 SQLite；API Key 使用 AES-256-GCM 加密后落库。

## 支持范围

- 多环境保存、编辑、删除、连通性测试与切换；
- Seedance 2.0 标准版、Fast 版和临时素材 API；
- Grok Imagine 图片生成、图片编辑、视频生成和视频编辑；
- Grok Imagine Video（`grok-imagine-video`）视频生成和视频编辑；
- Grok Imagine Video 1.5（`grok-imagine-video-1.5`）图生视频；
- 完整参数表单、请求 JSON 与 curl 实时预览；
- 服务端异步轮询、进度、结果预览、停止轮询和下载（停止 Demo 轮询不会取消上游任务或阻止计费）；
- 从目标 `/api/pricing` 动态计算预估费用；
- 从目标 `/api/log/token` 同步真实结算并展示差额；
- SQLite 请求/响应时间线，敏感请求头和签名参数自动脱敏。

## 启动

```bash
cd tools/molii-aigc-demo
cp .env.example .env
export MOLII_DEMO_MASTER_KEY="$(openssl rand -base64 32)"
GOWORK=off go run ./cmd/server
```

浏览器打开：<http://127.0.0.1:8787>

也可以使用参数覆盖环境变量：

```bash
GOWORK=off go run ./cmd/server \
  -addr 127.0.0.1:8787 \
  -db ./var/molii-aigc-demo.db
```

首次启动会创建 SQLite 数据库和所需表。`MOLII_DEMO_MASTER_KEY` 必须是 Base64 编码的 32 字节随机值；丢失后已保存的 API Key 无法恢复。

## 环境地址规则

- 生产与测试域名必须使用 HTTPS；
- 本地开发允许 `http://localhost:<port>`、`http://127.0.0.1:<port>` 和 `http://[::1]:<port>`；
- Base URL 不能包含用户名、密码、查询参数或 fragment；
- Demo 默认只监听 loopback，不应直接暴露到公网。

## 计费对比

预估费用根据目标环境实时返回的价格目录计算。异步任务成功后，Demo 使用同一个 API Key 请求 `/api/log/token`：

- Grok 图片按响应头 `X-Oneapi-Request-Id` 匹配；
- Seedance/Grok 视频按消费日志 `other.task_id` 匹配；
- 实际费用读取模型专用 billing snapshot，或按 New API 当前的计费单位（500000 quota / ¥）换算日志 quota。

日志尚未生成时页面显示“账单待同步”，不会用预估值冒充实际扣费。

## 安全说明

- 浏览器不使用 localStorage、sessionStorage、IndexedDB、Cache API 或 Service Worker；
- 浏览器不会收到已保存的明文 API Key；
- curl 预览始终使用 `$MOLII_API_KEY` 占位符；
- Authorization、Cookie、API Key 和带签名的查询参数不会写入请求历史；
- 图片结果的原始签名 URL 单独使用 AES-256-GCM 加密保存，页面通过受会话保护的同源入口跳转，不会把签名写入运行日志；
- Demo 不自动重试生成 POST，避免网络超时后产生重复付费任务；
- 进程重启时会从已落库的提交响应恢复异步 task；缺少可靠响应的中断请求只标记失败，绝不自动重放付费 POST；
- SQLite 目录与文件使用仅当前用户可读写权限。

## 测试

```bash
GOWORK=off go test ./...
GOWORK=off go test -race ./...
GOWORK=off go vet ./...
```

默认测试不会发起付费生成请求。

## macOS 后台开发服务

仓库提供 `scripts/watch-and-run.sh` 和 `scripts/sync-runtime-source.sh`。由于 macOS 不允许 LaunchAgent 直接读取 `Documents` 下的源码，前台同步守护进程会把变更复制到 `~/Library/Application Support/Molii/aigc-demo/source`，LaunchAgent 只读取该受保护的运行副本。它会监听 Go、HTML、CSS、JavaScript 和 Go module 文件；构建成功后原子替换二进制并重启服务，构建失败时继续保留旧进程。

当前机器的 `8787` 已由其他测试程序使用，因此常驻实例使用：<http://127.0.0.1:8788>。

页面每 2.5 秒检查一次服务实例版本。源码更新触发重启后，已打开的浏览器页面会自动刷新，不使用 localStorage 保存版本状态。
