# Grok 图片 WebUI 临时预览实施计划

## 全局约束

- 仅在 `develop` worktree 实施，不修改 `main`、生产配置或部署结构。
- 不恢复 COS 转存，不抓取图片，不改变 Grok 生成响应、计费或客户端错误契约。
- 原始 xAI 图片 URL 仅存 Redis 24 小时，不进入日志数据库、普通服务日志、错误信息或 localStorage。
- Redis 不可用时不得让已经付费成功的图片生成失败；API 仍返回原成功响应，只是不提供记录预览。
- 预览查询必须鉴权：普通用户只可访问自己的结果，管理员沿用现有日志查看权限。
- 所有结果 URL 必须再次通过现有可信 xAI 图片 URL 校验。
- 不发起真实付费请求。

## Task 1：后端临时预览索引与鉴权接口

文件归属：

- `service/grok_image_preview.go`
- `service/grok_image_preview_test.go`
- `relay/common/relay_info.go`
- `relay/channel/moliigrok/adaptor.go`
- `relay/channel/moliigrok/adaptor_test.go`
- `service/text_quota.go`
- `service/grok_image_billing_test.go`
- `controller/log.go`
- `controller/log_grok_image_preview_test.go`
- `router/api-router.go`

步骤：

1. 先写失败测试：Redis 24 小时存取、URL 可信校验、无 Redis 安全失败、owner/admin 权限、跨用户 404、过期/缺失、生成成功时 best-effort 注册、注册失败不改变成功响应、日志仅记录可用布尔值。
2. 使用 HMAC 化 Redis key，直接调用 Redis 客户端，禁止使用会在 debug 模式打印 value 的通用 helper。
3. 在 Grok 图片响应通过全部校验后，保存最多 4 个 URL；成功后给 relay info 设置非持久字段。
4. 计费日志仅写入 `grok_image_preview_available=true`，不写 URL。
5. 新增 UserAuth 保护的预览查询 API，以 `user_id + request_id` 查询；非 owner 且非 admin 返回 404。

验收：

- `go test ./service ./relay/channel/moliigrok ./controller -count=1`
- 相关 focused race test 通过。
- 无 URL、Redis 凭据或密钥进入日志/错误。

## Task 2：绘图记录详情预览卡片

文件归属：

- `web/src/features/usage-logs/api.ts`
- `web/src/features/usage-logs/types.ts`
- `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`
- `web/src/features/usage-logs/components/dialogs/grok-image-preview-card.tsx`
- 对应新测试文件
- `web/src/locales/en.json`
- `web/src/locales/zh.json`
- `web/src/locales/zh-TW.json`
- `web/src/locales/fr.json`
- `web/src/locales/ru.json`
- `web/src/locales/ja.json`
- `web/src/locales/vi.json`

步骤：

1. 先写失败测试：无 flag 不请求、有 flag 才请求、参数正确编码、加载/失效/错误态、1–4 张图、临时链接提示在图库之前。
2. 增加鉴权 API helper 和日志 Other 布尔字段。
3. 在 Grok 图片计费卡附近渲染预览卡；图片直连 xAI，使用 `referrerPolicy=no-referrer`，不持久化 URL。
4. 增加多语言临时链接与保存提醒。

验收：

- 相关 Bun 测试、typecheck、lint/format 检查通过。
- 无 flag 的历史记录不出现空预览组件。

## Task 3：整体验证、审查与发布

1. 运行后端全量 `go test ./... -count=1`、`go vet ./...`、`go build ./...`。
2. 运行前端相关测试与 typecheck；不执行生产构建。
3. 双模型审查工具若仍不可用，在 `review.md` 明确记录，并使用两个独立代码审查代理交叉审查。
4. 修复 Critical/Important 后归档 CCG task。
5. 提交并 push `origin/develop`，等待 Development CI/CD，验证 `/api/status` 与版本；不发起真实 Grok 请求。
