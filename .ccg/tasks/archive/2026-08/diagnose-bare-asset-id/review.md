# 诊断结果

- 请求把 Molii 临时素材 ID 直接作为 `image_url.url`，缺少 `asset://` 前缀。
- 适配器只解析 `asset://asset-molii-...`，裸 ID 会被当作普通上游 URL 并遭到拒绝。
- 正确值为 `asset://asset-molii-p8hirkbb2tetfcz5k9nesir1`。
- 用户在消息中暴露了完整 API Key，必须立即撤销并轮换。
