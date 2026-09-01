# 排行榜时间窗口与 Seedance 缺失诊断

## 时间范围

- 今天：当前时刻向前滚动 24 小时至当前时刻，按小时分桶。
- 本周：当前时刻向前滚动 7 天至当前时刻，按天分桶。
- 本月：当前时刻向前滚动 30 天至当前时刻，按天分桶。
- 今年：当前时刻向前滚动 365 天至当前时刻，按 7 天分桶。
- 这些并非自然日、自然周、自然月或自然年边界。

## 线上核对

- 已使用用户现有登录态读取 `https://dev.molii.co/rankings`。
- “今天”页面明确显示“过去 24 小时”，仅出现 `gpt-image-2`；ByteDance 分组为“暂无请求”。
- “本周”同样没有 Seedance 模型，ByteDance 分组为“暂无请求”。

## 根因

1. 模型/厂商排行榜聚合 `quota_data.token_used`，并排除 Token 合计为 0 的模型。
2. Seedance 终态日志虽然向 `RecordTaskBillingLog` 传入 `CompletionTokens`，但该函数写入 `LogQuotaData` 时遗漏 `TokenUsed`，导致异步任务统计行的 `token_used` 为 0，最终被排行榜过滤。
3. 分组成功率来源于 `perfmetrics.RecordRelaySample`；当前调用点只覆盖同步转发成功/失败路径，Seedance 异步任务终态没有记录性能样本，因此 ByteDance 分组没有请求数据。
4. 后端排行榜缓存为 5 分钟，前端 Query stale time 也为 5 分钟；即使修复入库，页面仍可能短暂延迟刷新。

本任务按用户要求只诊断，未修改排行榜代码。
