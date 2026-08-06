# 需求说明

## 目标

1. 后台“系统设置 → 运维 → 日志维护 → 清理日志”应同时清理普通日志和生成记录中的历史终态视频任务。
2. 模型广场中的四个 Grok Imagine 模型应展示符合 Molii 实际 API 契约的概览、性能、价格和 API 内容。

## 已确认边界

- 生成任务仅删除所选时间之前、状态为 `SUCCESS` 或 `FAILURE` 的记录。
- `NOT_START`、`SUBMITTED`、`QUEUED`、`IN_PROGRESS`、`UNKNOWN` 等非终态任务全部保留。
- Grok 图片性能不展示 TPS、TTFT、Token/s 或其他 Token 相关指标。
- Grok API 示例必须来自当前 Molii 后端真实路由和校验规则。
- 不影响 Seedance 和普通文本模型详情。
- 不调用 antigravity 或 Claude，不 push、合并或创建 PR。

## 验收范围

- 历史普通日志和终态生成任务均可被同一次清理任务删除，并分别报告数量。
- Grok 图片、视频模型拥有准确的中英文简介、能力概览和真实配置价格。
- Grok 图片性能展示请求量、平均响应时间和成功率；Grok 视频性能能够读取渠道 62 的真实任务数据。
- API 页覆盖图片生成/编辑、视频生成/编辑、状态查询和视频下载，并提供 cURL、Python、TypeScript、JavaScript 示例。
- 后端与前端均有对应自动化测试，并完成生产构建和浏览器验证。
