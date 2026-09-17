# Seedance 任务可观测性需求

## 范围

- `/usage-logs/common` 为当前用户可见的日志展示平台 `task_id`，支持复制；该 ID 本身已通过生成记录公开。
- Seedance 任务详情中的请求 ID 可跳转到使用日志并以该请求 ID 查询。
- `/usage-logs/task?source=seedance` 默认仍显示总耗时；管理员点击耗时后可查看提交、排队、生成、轮询检测等阶段的时间戳与耗时。
- Seedance 视频预览展示输入图片、视频、音频数量。

## 权限与隐私

- 详细耗时只放在管理员 DTO 中，普通用户接口不得返回。
- `upstream_task_id` 维持 Root-only，不扩大权限范围。
- 只持久化素材数量，不持久化或暴露 Prompt、素材 URL、Asset ID。
- StarAI 原始轮询响应不直接返回前端。

## 兼容性

- 新任务持久化标准化耗时快照和素材数量。
- 历史任务可从已脱敏的 StarAI 任务数据回退提取上游时间；不存在的数据展示“未记录”，不得伪造为零。
- 负耗时或时钟倒退不参与计算，展示为不可用。
- 不新增数据库表；复用 `tasks.private_data` JSON。

## 延后项

`model.claudeye.com` 的 BandwagonHost CI/CD、自建 PostgreSQL/Redis 与独立运行配置作为另一个任务处理，不进入本次变更。
