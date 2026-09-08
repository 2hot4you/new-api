# 临时素材 168 小时有效期

- 将 Molii Volcengine Imagine API 临时素材的默认生命周期从 24 小时改为 168 小时（7 天）。
- 新建的 Redis 素材映射、平台内 COS 素材清理时间和 API `expires_at` 保持一致。
- 页面和 Docusaurus 文档明确显示 168 小时，同时继续要求客户端以实际 `expires_at` 为准。
- 不修改与临时素材无关的 GPT Image、Grok 结果预览和 `/v1/files` 的 24 小时生命周期。
- 使用测试覆盖默认 TTL，避免未来误改回 24 小时。
