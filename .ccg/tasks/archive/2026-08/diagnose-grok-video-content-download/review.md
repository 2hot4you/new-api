# 审查结果

## 根因

- 公开任务状态为 `SUCCESS/100%`，保存的结果地址为 HTTPS。
- 结果源直接访问返回 `200 video/mp4`，内容长度为 2,420,073 字节。
- 本机代理 Fake-IP DNS 将官方视频域名解析到 `198.18.2.154`。
- 全局 SSRF 防护按设计拒绝 `198.18.0.0/15`，因此 Molii 下载代理返回 HTTP 403 JSON；`curl --output` 将 93 字节 JSON 保存成了 `.mp4`。

## 修复

- 新增 `IsTrustedMoliiGrokVideoURL`，只信任精确官方结果域名。
- 白名单同时要求 HTTPS、无 URL 用户信息、端口为空或 443；相似后缀、HTTP、自定义端口和其他域名均拒绝。
- 只有 Molii Grok 渠道且 URL 通过白名单时才绕过 Fake-IP 预校验并使用正常 relay client。
- 其他结果域名继续使用完整 SSRF 预校验和受保护拨号器；未关闭全局 SSRF。

## 安全审查

- 精确域名匹配使用 `url.URL.Hostname()`，不接受 `vidgen.x.ai.evil.example`。
- HTTPS 由 Go TLS 校验保护，未关闭证书校验。
- 渠道密钥、上游任务 ID 和结果 URL 不进入用户错误响应。
- 用户明确要求不调用 antigravity 或 Claude，因此未执行 CCG 外部双模型审查，改为 Codex 直接审查完整 diff 和 SSRF 边界。

## 验证证据

- TDD：白名单安全测试先因函数不存在而失败，最小实现后通过。
- `go test ./...`：通过。
- Range 实测：HTTP 206、`video/mp4`、1024 字节，`Content-Range: bytes 0-1023/2420073`。
- 完整下载实测：HTTP 200、`video/mp4`、2,420,073 字节。
- `file` 将下载结果识别为 ISO Base Media MP4。
- launchd `com.molii.new-api` 常驻运行，健康接口返回 200。
