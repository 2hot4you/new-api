# Review

## Root cause

统一 `/v1/videos` 端点的身份过滤已把类型 61 加入 Doubao `openai_video` 请求的允许渠道类型，但 `SetupContextForSelectedChannel` 随后又按插件原始 `channelTypes`（54/45）执行第二次校验，因此拒绝 StarAI 61。`Distribute` 还忽略了该初始化错误，继续使用空渠道上下文，最终由 JS 插件报出误导性的 `plugin_request_invalid`。

## Changes

- 导出并复用当前请求唯一的任务插件渠道类型允许列表，让筛选和初始化校验口径一致。
- 仅为受支持的四个 Seedance 2.x `openai_video` 模型允许 StarAI 61；无关原生渠道继续被拒绝。
- 已选渠道初始化失败时立即中止并返回原始错误码。
- `channel == nil` 的合法任务查询路由不执行渠道初始化，保持原行为。

## Verification

- RED：新增两项回归测试后，StarAI 61 被二次拒绝，初始化错误被吞掉。
- GREEN：两项测试修复后通过。
- 兼容回归：`TestGetOpenAIVideoRouteRendersJimengTask` 通过。
- 相关包：`go test ./service ./middleware ./relay/... -count=1` 通过。
- 全量：`go test ./... -count=1` 通过。
- 独立审查：无 Critical/Warning，确认 StarAI 放行范围和 nil-channel 查询流程正确。

## External model limitation

CCG 要求的 antigravity 与 Claude wrapper 在本机不存在（`~/.claude/bin/codeagent-wrapper`: exit 127），无法执行外部双模型分析；已使用失败回归测试、全量测试和独立审查代理验证。
