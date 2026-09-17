# 排查结论

## 根因

1. `TaskDetailsDialog` 已实现 root 诊断展示，但没有被 `task-logs-columns.tsx` 或任务表格其他入口实例化，因此 Seedance 生成记录页面无法打开该弹窗。
2. 日志页面的 root 能力取决于视图范围。超级管理员选择“仅自己”时，前端把访问级别降为 `self`，后端走 `/api/task/self` 或用户日志接口并移除 `root_info`。
3. 通用日志仅对完成结算后写入的日志保存 `root_info.upstream_task_id`；历史记录不会自动回填。

## 已确认的正常链路

- StarAI 提交响应的上游任务 ID 写入 `Task.PrivateData.UpstreamTaskID`。
- root 访问全部任务接口时，后端投影为 `root_info.upstream_task_id`。
- root 访问全部日志接口时，`FormatRootLogs` 保留 `root_info`。

## 工具限制

antigravity 与 Claude 并行分析均已尝试，但 `/Users/naf/.claude/bin/codeagent-wrapper` 不存在，两个调用均以 127 退出；未声称完成外部双模型分析。
