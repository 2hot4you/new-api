# 需求

- 调查 Grok Imagine Video 上游已失败，但 Molii 任务仍显示“待确认”的根因。
- 确保上游失败状态和错误信息能够正确同步、持久化并在任务详情或日志中展示。
- `/usage-logs/task?source=grok-video&page=1` 列表补充“令牌”和“模型”字段。
- Grok Video 存在多个模型 ID，模型字段必须展示实际 `model_id`，不能仅由分类名代替。
- 列表字段样式参照现有图像生成任务列表。
- 修复必须有能够先失败再通过的回归测试。
