# Requirements

- 不修改 `billing_expr`、`matched_tier`、档位内部标识或后端结算逻辑。
- `len` 条件档位优先显示真实单次输入 Token 范围，不展示 `short_context`、`long_context` 等内部名称。
- 仅有基础档并叠加时段规则时，将 `base` 显示为“基础时段价格”，并明确未命中特殊时段时适用。
- 其他无条件基础档将 `base`/`default` 显示为“默认价格”。
- 模型广场详情、分组价格表、使用日志摘要和使用日志详情使用同一套语义标签规则。
- 所有新增文案同步现有语言；历史日志仍通过内部原始标识完成匹配。

# Analysis note

CCG 要求的 antigravity 与 Claude 并行分析均已尝试，但当前环境不存在 `~/.claude/bin/codeagent-wrapper`，两个调用均以 127 退出。后续采用本地 TDD、类型检查和逐文件审查补偿。
