# 分析结论

- 复杂度：M；风险：中；领域：fullstack。
- 后端 `buildModelHistory` 当前仅保留前 10 个模型，并把其余模型按桶合并为 `Others`。
- 前端 `ModelsSection` 再次把维度 Tooltip 截断为前 10 行，并把其余行合并为“还有 N 项”。
- 排行榜主体模型列表仍限制为 20 个，因此 Tooltip 图标不能只从 `rows` 构建；需要让历史序列直接携带 metadata 中的 `model_icon`。
- VChart 支持 `tooltip.style.maxContentHeight`，超限后由组件内部设置纵向滚动。
- 供应商市场份额使用独立的 `buildVendorShareHistory` 与 `rankingVendorLimit`，本任务不修改。

## 双模型分析

- antigravity：调用失败，`codeagent-wrapper` 不存在，退出码 127。
- Claude：调用失败，`codeagent-wrapper` 不存在，退出码 127。
- 在外部模型不可用的情况下，按仓库现有实现、类型和依赖源码完成了本地分析。
