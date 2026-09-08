# Review

## Root cause

`POST /v1/videos` 的请求正文合法。实际失败发生在上游请求构建阶段：旧渠道或复制渠道可能将 `base_url` 保存为 SQL `NULL`，而 `Channel.GetBaseURL` 只会为空字符串回退默认地址。Doubao 插件因此构造出 `/api/v3/...` 相对路径，并由 URL 安全校验返回 `plugin_request_invalid`。

## Change

- `base_url == nil` 时使用渠道类型的内置默认地址。
- 自定义插件等没有内置地址的渠道仍得到空字符串，不改变必须显式配置 Base URL 的约束。
- 增加 NULL 默认地址回退及自定义渠道不回退的回归测试。

## Verification

- TDD RED：`go test ./model -run 'TestChannelGetBaseURL' -count=1` 在修复前按预期失败。
- TDD GREEN：同一命令修复后通过。
- 相关包：`go test ./model ./middleware ./relay/... -count=1` 通过。
- 全量 Go：`go test ./... -count=1` 通过。
- 独立代码审查：无 Critical/Warning；确认显式自定义地址和无默认地址渠道行为不变。

## External model limitation

CCG 要求的 antigravity 与 Claude wrapper 在本机不存在（`~/.claude/bin/codeagent-wrapper`: exit 127），因此无法执行外部双模型分析；已使用本地回归测试、全量测试和独立审查代理补足验证。
