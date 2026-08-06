# Grok 视频编辑任务类型与预览设计

## 背景与根因

任务 `task_rR90qPDjBcnNcJP7cOGvcu2NhADWecJw` 的持久化数据为：平台 `62`、动作 `video_edit`、状态 `SUCCESS`，并且存在私有结果视频 URL。

当前前端存在两个缺口：

1. `TASK_ACTIONS` 和 `TASK_ACTION_MAPPINGS` 没有 `video_edit`，动作映射回退为“未知”。
2. `TaskProgressCell` 只把平台 `61` 视为可预览的视频任务，因此平台 `62` 即使已经成功且 API 返回 `result_url`，也不会显示“已生成”和“预览”。

后端已经完整支持 Grok 视频安全预览：成功任务的私有上游 URL 会转换为 Molii 的 24 小时签名代理地址 `/v1/videos/{task_id}/content?...`，任务列表刷新时会重新生成签名。现有控制器测试也覆盖了平台 `62` 不泄露上游 URL 的行为，因此不需要修改后端 API、数据库或代理。

## 方案

采用最小前端修复并复用 Seedance 现有预览体验。

- 在 `TASK_ACTIONS` 增加 `VIDEO_EDIT: 'video_edit'`。
- 在 `TASK_ACTION_MAPPINGS` 增加 `video_edit → Video Editing`，中文沿用现有“视频编辑”翻译。
- 在 `TASK_PLATFORMS` 增加 `MOLII_GROK: '62'`，平台显示标签为 `Grok`。
- 抽取一个纯函数判断任务是否属于已生成视频：平台为 `61` 或 `62`，且状态为 `SUCCESS`。
- `TaskProgressCell` 使用该判断；存在 `result_url` 时显示“预览”按钮并复用 `VideoPreviewDialog`。
- 没有 `result_url` 的成功任务只显示“已生成”，不渲染不可用按钮。
- 非成功任务保持当前进度显示。

## 安全与错误处理

- 前端只接收并使用 Molii 签名代理地址，不读取或展示 `private_data.result_url`。
- 视频 URL 仍通过现有 `/v1/videos/{task_id}/content` 权限、签名和 SSRF 防护链路获取。
- 签名过期或视频播放失败时，沿用现有弹框错误提示；刷新任务列表即可取得新签名。
- 不持久化新的 URL 或任务字段。

## 测试与验收

- 动作 `video_edit` 映射为 `Video Editing`。
- 平台 `62` 映射为 `Grok`。
- 平台 `62`、`SUCCESS`、存在 `result_url` 时显示“已生成”和“预览”。
- 平台 `62` 成功但缺少 `result_url` 时不显示预览按钮。
- 平台 `62` 非成功状态不进入已生成预览分支。
- 平台 `61` 的现有预览行为保持不变。
- 浏览器在指定任务行验证“Grok · 视频编辑”，点击“预览”后打开现有视频弹框并加载签名代理地址。
