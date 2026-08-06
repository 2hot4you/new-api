# 审查记录

## 结论

- 图片路由确认：`POST /v1/images/generations`、`POST /v1/images/edits`。
- 视频路由确认：`POST /v1/videos`、`POST /v1/videos/edits`、`GET /v1/videos/:task_id`、`GET /v1/videos/:task_id/content`。
- Token 日志路由确认：`GET /api/log/token`，使用同一 Bearer Token，可读取该 Token 最近的日志。
- 请求关联字段确认：响应头 `X-Oneapi-Request-Id` 对应日志字段 `request_id`。
- 图片输入必须是可由上游访问的 URL 或 `file_id`；示例使用公开 HTTPS URL 占位符。
- 未发现示例中包含密钥或持久化测试凭据。

## 验证

执行：

`go test ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./controller ./router`

结果：全部通过。

