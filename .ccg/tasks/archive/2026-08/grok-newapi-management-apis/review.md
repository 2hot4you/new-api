# Grok New API 管理接口分析结论

## 真实只读联调

- 系统访问令牌调用 `GET https://api.wxiai.com/api/user/self` 时，上游要求 `Authorization: Bearer <PAT>` 与 `User-id: 2205`；请求成功并返回用户 2205 的 quota。
- 相同鉴权调用 `/api/user/models` 成功，返回 76 个用户可用模型。
- 渠道 Key 调用根路径 `/v1/models` 成功，返回 11 个该 Key 实际可用模型。
- `/xai/v1/models` 与渠道 Key 的 `/api/usage/token/` 均返回 404。
- `/api/status` 返回 `quota_per_unit: 500000`，余额实现必须动态读取该值。

## 实施归并

- 余额采用系统访问令牌、`User-id`、`/api/status` 与 `/api/user/self`。
- 模型采用渠道 Key 与管理根路径 `/v1/models`。
- 系统访问令牌通过环境变量注入，不写入渠道 JSON、源码或示例真实值。
- 实施已归并至 `.ccg/tasks/rename-molii-aigc-channel` 与对应设计/计划。

## 安全

- 联调输出仅包含 HTTP 状态、quota 字段和模型 ID，没有回显令牌。
- 临时响应文件在请求后删除，没有写入仓库。
- 按用户既有要求未调用 antigravity 或 Claude。
