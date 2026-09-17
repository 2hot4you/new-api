# 修复 StarAI 上游任务 ID 可见性

## 验收标准

- 持久化异步结算日志保留 root-only 的 `root_info.upstream_task_id`。
- 普通用户与普通管理员接口仍剥离该字段。
- Seedance 任务记录提供可打开的任务详情入口。
- 超级管理员“全部”视图能在任务详情看到 StarAI 上游任务 ID。
- “仅自己”视图继续按普通用户权限隐藏 root 诊断。
- 不向普通用户、普通管理员或公开任务响应泄露上游任务 ID。

## 根因

- `recordTaskBillingReconciliationEvent` 在构造日志前主动清空 `UpstreamTaskID`。
- `TaskDetailsDialog` 已实现但未挂载到任务表格。

## 工具限制

antigravity 与 Claude 外部分析工具当前不可用，两个并行调用均以 127 退出。
