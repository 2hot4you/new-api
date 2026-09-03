# 审查结果

## 结论

未发现 Critical 问题。正常 API 与敏感操作的边界符合已确认设计：全局 Web/API 与搜索限流默认关闭；refresh/logout、普通 usage/log 查询不再使用 Critical；匿名敏感动作按用途和 IP 隔离；已认证敏感动作按用途和用户 ID 隔离。

## Warning

- 所有默认未显式配置限流环境变量的环境都会采用新策略。如果某个部署显式设置了 `GLOBAL_API_RATE_LIMIT_ENABLE=true`、`GLOBAL_WEB_RATE_LIMIT_ENABLE=true`、`SEARCH_RATE_LIMIT_ENABLE=true` 或旧的 Critical 阈值，环境变量仍会按既有契约覆盖代码默认值，部署时应同步移除或更新这些覆盖项。
- 登录仍受数据库级 Session 签发上限保护；这属于防止 Session 表无界增长的独立安全机制，不与 IP Critical 桶共享，也不在本次范围内移除。

## 测试与验证

- TDD RED：`go test ./middleware -run TestCriticalRateLimitUsesIndependentIPScopes -count=1` 在旧接口上因缺少 scope 参数编译失败。
- TDD GREEN：同一测试及现有限流关键测试通过。
- `go test ./middleware ./router -count=1` 通过。
- `go test ./... -count=1` 通过，所有 Go 包零失败。
- `git diff --check` 通过。

## 外部审查

按 CCG 要求并行尝试 antigravity 与 Claude 分析和审查，但本机缺少 `/Users/naf/.claude/bin/codeagent-wrapper`，四次调用均退出 127，未取得外部模型报告。已完成本地逐项审查及全量测试。

## Spec Evolution

已更新 `docs/authentication.md`，记录默认关闭全局限流、敏感动作隔离键及 Critical 新默认阈值，避免后续部署继续依据旧的 `20 次/20 分钟` 说明。
