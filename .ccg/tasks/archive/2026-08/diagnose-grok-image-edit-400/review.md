# 审查结果

## 根因

- Molii Token 校验、本地 JSON 解析、图片 URL 可访问性和 `/v1/images/edits` 路由均正常。
- 受控诊断确认上游以 `imagine:content-moderated` 拒绝原始素材/提示词组合。
- 原有脱敏器将所有上游 400 统一映射成 `bad_response`，导致用户误以为后端故障。
- URL 图片对象未补齐官方格式中的 `type: image_url`，属于请求兼容性缺口，但不是本次内容审核拒绝的根因。

## 变更

- URL 图片输入统一向上游发送 `type: image_url`；`file_id` 输入不附加该字段。
- 新增稳定错误码 `content_policy_violation`。
- 只在精确匹配上游内容审核错误码时返回安全的 Molii 提示；其余上游错误继续使用通用脱敏响应。
- 回归测试覆盖官方图片编辑请求体和内容审核错误映射。

## 安全审查

- 用户响应不包含渠道密钥、上游域名、原始消息或上游 Request-ID。
- 内容审核错误标记为不可重试，避免无意义的渠道重试。
- 用户明确要求不调用 antigravity 或 Claude，因此未执行 CCG 外部双模型审查，改为 Codex 直接审查完整 diff 和脱敏边界。

## 验证证据

- TDD：请求体测试先因缺少 `type` 失败，修复后通过；内容审核映射测试先因返回 `bad_response` 失败，修复后通过。
- `go test ./...`：通过。
- `go build`：通过。
- 改动 Go 文件 `gofmt -l`：无输出。
- `git diff --check`：通过。
- 原始请求回归：HTTP 400，`type/code = content_policy_violation`，无上游敏感信息。
- 官方公开图片与中性提示词回归：HTTP 200，返回 1 张有效图片。
- launchd `com.molii.new-api`：running，健康接口返回 200，端口 3000 正常。
