# StarAI 临时素材失败原因

- 解析上游 HTTP 200、业务状态 `FAILED` 响应中的 `error.code` 与 `error.message`。
- 将脱敏后的失败信息随临时素材 Redis 映射保存，并在后续刷新中更新或清除。
- `GET /v1/assets/{id}` 及后台素材接口返回结构化 `error` 对象。
- `/temporary-assets` 卡片展示失败代码与失败原因。
- 素材被生成任务引用时，返回真实审核失败原因。
- 更新 API 文档与相关后端、前端测试。
- 不改变素材 ID、168 小时有效期、COS 或渠道选择逻辑。
