# 文档审查

## 覆盖范围

- 正式域名、API Key 和控制台会话的区别。
- 模型发现、OpenAI/Anthropic/Gemini 兼容入口。
- 文本、图片、嵌入、音频、重排接口。
- Seedance 2.0 两个内置模型、请求字段、场景限制、任务轮询和视频下载。
- StarAI 临时素材、API Key 用量、错误码、重试与安全建议。
- 只包含用户接口，不包含管理员接口或真实凭据。

## 源码核对

- 路由：`router/relay-router.go`、`router/video-router.go`、`router/api-router.go`。
- 认证：`middleware/auth.go`。
- 视频 DTO 与任务响应：`relay/common/relay_info.go`、`relaykit/dto/openai_video.go`、`relay/relay_task.go`。
- StarAI 请求约束：`relay/channel/task/starai/adaptor.go`、`constants.go`。
- 素材和视频代理：`controller/starai_asset.go`、`controller/video_proxy.go`。
- 用量查询：`controller/token.go`。

## 验证

- Markdown 共 821 行，86 个围栏标记，代码围栏成对。
- 文档内相对认证文档存在。
- Compose 示例配置解析通过。
- `git diff --check` 通过。
- `go test ./router ./relay/channel/task/starai ./controller` 通过。
- 线上 `/api/status` 返回 200；无令牌访问 `/v1/models` 返回 401。
- 按用户要求未调用 antigravity 或 Claude。
