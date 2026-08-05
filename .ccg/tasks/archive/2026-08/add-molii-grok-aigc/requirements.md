# Molii Grok Imagine API 渠道需求

## 范围

- 新增独立渠道类型 `Molii Grok Imagine API`，不修改或复用 Molii AIGC 与官方 xAI 渠道业务逻辑。
- 支持两个图片模型和一个异步视频模型。
- 接入管理后台渠道配置、模型目录、固定价格计费、错误脱敏与测试。

## 核心约束

- 渠道类型 ID 为 62，Dummy 顺延为 63，其他 ID 保持不变。
- 默认上游地址仅存在于服务端；前端和普通用户响应不得出现上游品牌、域名、请求 ID、Key、原始错误或结果 URL。
- 图片按成功响应中的实际图片数量结算，视频按管理员配置的固定单次价格计费。
- 视频提交不自动跨渠道重试，HTTP 202 轮询响应为正常进行中状态。
- 视频结果只通过公共 content URL 和现有 SSRF 防护代理输出，且仅接受 HTTPS。
- 使用项目 `common.Marshal` / `common.Unmarshal`，不硬编码密钥，不修改 web 之外的独立 Molii 前端。

## 验收

- 覆盖附件列出的后端与前端测试场景。
- 通过 `go test ./...`、`go build ./...`、`bun run i18n:sync` 与 `bun run build`。
