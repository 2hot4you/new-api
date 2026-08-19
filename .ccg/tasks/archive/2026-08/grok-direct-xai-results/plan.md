# 实施计划

## 目标

停止 Molii Grok Imagine 图片和视频结果的 COS 转存。图片同步响应直返经严格校验的 xAI 临时 URL；视频任务保存经校验的临时 URL，标准任务查询直返 URL，受保护的内容接口返回 307。Grok 视频生成记录预览明确提示链接临时性。

## Task 1：集中式 xAI 结果 URL 校验与图片直返

文件范围：

- `service/molii_grok_result_security.go`
- `service/molii_grok_result_security_test.go`
- `relay/channel/moliigrok/adaptor.go`
- `relay/channel/moliigrok/adaptor_test.go`

步骤：

1. 先增加失败测试，覆盖图片/视频精确主机、HTTPS、userinfo、端口、伪造后缀、畸形 URL，以及三种图片模型、多图、计费和不调用持久化。
2. 新增集中式纯 URL 校验，不发起网络探测，不在错误中包含原 URL。
3. 图片响应逐项校验后直接构造客户端 DTO，保留 MIME、revised prompt 与实际输出计费。
4. 非法 URL 整批失败并维持通用安全 502。

## Task 2：视频直返、私有结果保存与安全 307

文件范围：

- `relay/channel/task/moliigrok/adaptor.go`
- `relay/channel/task/moliigrok/adaptor_test.go`
- `service/task_polling.go`
- `service/task_polling_molii_grok_test.go`
- `controller/video_proxy.go`
- `controller/video_proxy_molii_grok_test.go`
- `controller/task.go`
- `controller/task_video_preview_test.go`
- `relay/relay_task.go`
- `relay/relay_task_video_url_test.go`

步骤：

1. 先增加失败测试，覆盖轮询结果校验、无 COS 完成、private result URL、计费不变、任务查询直返和内容接口 307。
2. 删除 Grok 终态 COS 持久化调用，把已校验 URL 保存到 `PrivateData.ResultURL`，公开轮询快照不得包含 URL。
3. 标准视频任务查询的 `metadata.url` 返回已复验的 xAI URL。
4. 普通用户生成记录仍使用受保护的同源 content URL，以便播放器经鉴权/签名后 307；不把完整 URL写入 usage log。
5. Grok content 入口在归属和成功状态校验后，从私有字段取 URL，复验后以空响应体 307 跳转；设置 `Cache-Control: private, no-store`、`Referrer-Policy: no-referrer` 和 `X-Content-Type-Options: nosniff`。
6. Grok 新行为忽略历史 `StoredResult`；Seedance 和其他平台保持原代理/存储逻辑。

## Task 3：Grok 视频预览临时链接提示

文件范围：

- `web/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx`
- `web/src/features/usage-logs/lib/task-video-preview.ts`
- `web/src/features/usage-logs/lib/__tests__/task-video-preview.test.ts`
- `web/src/i18n/locales/en.json`
- `web/src/i18n/locales/zh.json`
- `web/src/i18n/locales/zh-TW.json`
- `web/src/i18n/locales/fr.json`
- `web/src/i18n/locales/ru.json`
- `web/src/i18n/locales/ja.json`
- `web/src/i18n/locales/vi.json`
- `docs-site/docs/models/grok-imagine-image.mdx`
- `docs-site/docs/models/grok-imagine-video.mdx`
- `docs-site/docs/api-reference/videos.mdx`
- `docs-site/docs/examples/grok-poll-download.mdx`
- `docs-site/docs/platform/usage-and-generation-records.mdx`
- `docs-site/scripts/grok-content-contract.test.ts`

步骤：

1. 先增加失败测试，明确只有 Grok 视频平台显示临时链接提示。
2. 在播放器上方使用现有提示组件显示“临时链接可能过期，请及时下载保存”。
3. 同步七种语言，不改图片日志契约、不把结果 URL写入本地存储或日志。
4. 更新用户文档中已过时的“Molii COS 保存 24 小时”和内容代理说明：明确 Grok 结果由 xAI 临时提供、任务查询直返视频 URL、content 入口为受保护的 307，下载时不要把 Molii API Key 转发到目标域名。

## Task 4：验证、审查、归档和发布

1. 运行相关 Go 聚焦测试、`go test ./... -count=1`、`go vet ./...`、`go build ./...`。
2. 运行前端聚焦测试、i18n 检查、typecheck 和相关 lint；不做生产前端构建。
3. 检查 gofmt、diff、敏感信息和客户端错误契约。
4. 进行独立代码审查；由于本机 CCG 外部双模型 wrapper 缺失，在 `review.md` 记录，并用可用独立审查代理补充。
5. 更新 task 状态、归档 CCG 任务、提交并推送 `origin/develop`。
6. 等待 Development CI/CD，仅验证 Actions 与 `/api/status`，不发起真实 Grok 生成请求。
