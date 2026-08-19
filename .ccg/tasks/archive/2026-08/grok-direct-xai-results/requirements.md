# Grok 临时结果直返

## 范围

- 仅调整 Molii Grok Imagine 图片和视频模型的结果交付链路。
- 图片成功后严格校验并直接返回 xAI 官方临时 URL，不再转存 COS。
- 视频完成后严格校验并保存 xAI 官方临时 URL；任务查询直接返回该 URL。
- `GET /v1/videos/{task_id}/content` 对合法 Grok 视频结果返回安全 `307 Temporary Redirect`。
- Grok 不考虑历史 StoredResult 兼容；不迁移、删除或清理历史数据。
- Grok 视频生成记录的预览界面提示临时链接可能过期，建议用户及时下载保存。
- Grok 图片仍只在同步 API 成功响应中返回，不为提示而新增图片 URL 日志字段或历史预览契约。

## 安全边界

- URL 仅允许 HTTPS、无 userinfo、无自定义端口，并精确匹配对应 xAI hostname allowlist。
- 不对结果 URL 发起服务端 GET、HEAD 或 DNS 探测。
- 不记录完整 URL、path、query、Token、渠道密钥或存储凭据。
- 307 目标只能来自已鉴权、已归属且成功的 Grok 任务结果，禁止开放重定向。

## 行为约束

- 保留图片输出数量、MIME、revised prompt、实际计费和 `grok_image_billing`。
- 保留视频异步状态、进度、时长、分辨率、最终结算和 `grok_video_billing`。
- 不改变 Seedance、其他图片/视频渠道、全局 COS 或对象存储能力。
- 不执行真实付费请求；完成后提交并推送 `origin/develop`，仅验证 Development CI/CD 和健康状态。
