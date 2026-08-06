# Grok 视频播放失败修复审查

## 根因证据

- 本地数据库 `ServerAddress` 为 `https://aigc.molii.co`。
- Chrome 页面运行在 `http://localhost:3000`，点击预览时本地后端没有收到 `/content` 请求。
- 远端 `aigc.molii.co:443` 无法建立 TLS 连接。
- 使用本地 API 密钥直接请求同一任务时，代理返回 206、有效 MP4 文件头和 2,356,602 字节完整文件；Quick Look 可解码。

## 修复

- 新增同源签名路径构造函数，仅供后台任务列表使用。
- StarAI 与 Molii Grok 成功任务的后台预览 URL 改为相对路径。
- 用户侧视频任务 API 继续使用绝对 URL，外部 API 契约不变。
- 签名算法、用户归属和有效期不变。

## 审查结论

- Critical：无。
- Warning：无。
- 浏览器复验：同一任务正常显示首帧与 5 秒播放控件，本地代理返回 206，控制台无错误。
- `go test ./...`：通过。
- `go vet ./...`：通过。
- `gofmt -d` 与 `git diff --check`：通过。
- LaunchAgent `com.molii.new-api`：运行中；`GET /api/status` 返回 200。
- 按用户既有要求未调用 antigravity 或 Claude。
