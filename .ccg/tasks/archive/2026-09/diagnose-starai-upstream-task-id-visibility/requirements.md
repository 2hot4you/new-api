# StarAI 上游任务 ID 可见性排查

## 问题

超级管理员登录后，在 Seedance 任务详情中看不到 StarAI `upstream_task_id`。

## 排查目标

- 确认登录角色是否被前端和后端识别为 root。
- 确认任务列表接口是否返回 `root_info.upstream_task_id`。
- 确认任务记录是否持久化 `private_data.upstream_task_id`。
- 确认前端详情弹窗是否在正确入口展示根级诊断。
- 区分 StarAI `task_id` 与下游火山 `upstream_id`。

## 限制

本阶段只诊断，不修改线上数据或代码。
