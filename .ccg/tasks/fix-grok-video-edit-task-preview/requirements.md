# 需求

- 指定任务 `task_rR90qPDjBcnNcJP7cOGvcu2NhADWecJw` 在“视频生成 → Grok Video”中应识别为视频编辑，而不是未知。
- Grok Video 成功任务应与 Seedance 成功任务一致，显示“已生成”和“预览”入口。
- 点击预览打开现有视频预览弹框并播放任务结果。
- 不向前端暴露上游私有视频 URL、上游任务 ID 或签名信息。
- 复用 Molii 已有的短期签名视频代理 URL，不新增持久化字段或数据库迁移。
- 对任务类型映射和 Grok Video 预览资格补充回归测试。
