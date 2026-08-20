# Review

## Scope

- 首页厂商跑马灯遵循供应商 `display_order`，两条跑马灯保持相同业务顺序。
- 首页搜索仅匹配模型 ID 与厂商名称，取消六条上限并提供内部滚动。
- 从 `billing_mode`/`billing_expr` 推导按单次输入 Token 与按请求时段的计价说明。
- 只修改前端展示、解析辅助函数、测试与 i18n；未修改后端计费计算。

## Findings

- Critical: 0
- Warning: 0
- Info: 1 — 对无法安全结构化识别的请求条件继续使用通用“动态计价”展示，避免误导用户。

## Verification

- `bun test src/features/home`: 32 passed, 0 failed.
- `bun test src/features/pricing`: 89 passed, 0 failed.
- `bun run typecheck`: passed.
- `bun run i18n:check`: passed.
- scoped `oxlint`: passed.
- scoped `oxfmt --check`: passed.
- `git diff --check`: passed.
- 本地 `127.0.0.1:3000` 浏览器验证：单字符 `g` 返回匹配模型；能力词“视频”不误命中；结果列表标记为内部可滚动；供应商跑马灯两条使用相同顺序。

## Review limitation

仓库要求的 antigravity/Claude 外部双模型 wrapper 在当前环境不可用（`~/.claude/bin/codeagent-wrapper` 不存在），因此本轮无法执行外部双模型审查；已完成逐文件人工审查与上述自动化验证。

## Security and behavior

- 未引入密钥、凭据或用户数据。
- 未改变计费表达式、价格计算、请求参数或 API 契约。
- 未运行生产构建、未提交、未推送、未操作部署环境。
