# Grok 图片 WebUI 预览审查

## 结果

- Spec compliance：通过
- Code quality：通过
- Critical：0
- Important：0
- Minor：1（非阻断）

## 已解决问题

1. 预览 API 的 200、404、401 响应均禁止浏览器和中间缓存。
2. Redis SET/GET、TCP、TLS 握手、读写、连接池等待均限制为 200ms，且禁用 go-redis 默认重试；Redis 异常不改变已付费生成的成功响应。
3. 鉴权测试穿过真实 Gin 路由、`UserAuth` 与 PAT，覆盖未登录、owner、跨用户与 admin。
4. 前端安全处理畸形响应、404 过期、500/网络异常，不展示或 toast 原始错误正文。
5. 后台重新查询时立即移除缩略图与已经打开的旧图片弹窗，404/500 后不会恢复过期 URL。
6. 缩略图和大图均使用 `referrerPolicy=no-referrer`；URL 不写 localStorage。
7. 组合测试的 happy-dom/React Query 状态隔离已修复，连续组合运行稳定。

## 非阻断事项

- 当前已有分层测试覆盖响应适配、Redis、计费 Other 布尔字段、真实路由鉴权和前端查询，但尚无单个测试从上游响应一路贯穿到真实消费日志持久化再访问预览 API。静态检查确认各层使用相同的 `user_id + request_id`，本轮不扩大测试基础设施范围。

## 安全边界

- xAI 临时 URL 仅存 Redis，TTL 24 小时。
- Redis key 使用安装级 HMAC，不包含明文用户 ID 或 request ID。
- 日志数据库只存 `grok_image_preview_available=true`，不存 URL。
- 预览仅 owner/admin 可访问；跨用户、缺失、过期和 Redis 异常统一 404。
- 不恢复 COS、不做服务端图片下载、不改计费、不改图片生成响应。

## 外部双模型审查

按 CCG 要求尝试调用 antigravity 与 Claude，但本机缺少 `/Users/naf/.claude/bin/codeagent-wrapper`，两者均无法启动。已使用两个独立代码审查代理分别从后端安全和前端契约角度交叉审查，并完成三轮后端修复、两轮前端修复和最终整体验证。

## 验证

- `go test ./... -count=1`：通过
- `go vet ./...`：通过
- `go build ./...`：通过
- 后端 focused/race（service、moliigrok、controller、router）：通过
- 前端组合测试：22 passed，0 failed
- `bun run typecheck`：通过
- `bun run i18n:check`：通过
- 变更文件 scoped Oxlint/Oxfmt：通过
- `gofmt -d`：无输出
- `git diff --check`：通过
