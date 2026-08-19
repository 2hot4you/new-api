# Implementation Plan

## 根因确认

1. 对照 Development 部署日志、官方响应契约和当前代码路径。
2. 记录当前可证实事实：上游请求成功后，`DoResponse` 才进入结果持久化；当前图片源校验在任何 COS 请求前强制要求非空且受支持的 `mime_type`，而真实错误被适配器统一丢弃为 502。
3. 新增证据显示旧模型同样失败，因此不把根因限定为 Image 2.0 或 MIME。由于现有 GitHub Actions 仅含部署健康日志、当前环境没有 Development 容器 SSH 入口，不臆造旧容器内部错误；第一阶段只加入阶段化日志，供部署后唯一一次真实请求确认。

## 实施

1. 新增安全的 Grok 图片持久化阶段错误，只公开固定 stage、error_category 与解析后的 source host，不携带原始 URL、查询参数、COS 签名或底层 SDK 错误文本。
2. 在既有解析、源 URL/MIME 校验、Redis 锁、对象键、COS HEAD、清理队列、远程抓取/重定向/类型/大小、COS PUT/签名边界包装真实 cause；不改变分支条件与返回行为。
3. 在适配器失败处输出 request_id、model、user_id、channel_id、stage、error_category、source_host 的结构化脱敏错误日志；客户端继续只收到通用 502。
4. 保持成功后的 Molii COS URL响应和 `grok_image_billing` 最终分项更新。

## TDD 与验证

1. 先新增失败测试：每个规定 stage 的稳定分类、实际 cause 保留、日志不泄露 URL/查询参数/签名/密钥、通用客户端错误、计费和三模型成功回归。
2. 运行定向 RED，实施日志最小修复后运行 service、moliigrok、controller 和全量 Go 测试。
3. 执行 gofmt、go vet、git diff --check 和敏感信息扫描。
4. 独立审查变更，修复 Critical/Warning 后提交并推送 `origin/develop`。
5. 观察 GitHub Actions 和 Development 健康检查。部署后由用户只运行一次真实 `grok-imagine-image` 请求并回传 request ID/脱敏日志，再进入第二阶段修复。
