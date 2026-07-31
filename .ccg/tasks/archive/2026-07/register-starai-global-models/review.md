# 审查结果

- 两个 Seedance 2.0 模型从 StarAI TaskAdaptor 单一来源注册到 `openAIModels`，避免后端重复维护模型字符串。
- `/api/channel/models` 返回的模型 owner 为 `starai`，每个模型只出现一次。
- 原有 `/api/models` 渠道类型映射继续复用同一个 adaptor。
- 未增加或推测任何默认 ModelRatio/ModelPrice。
- 定向 controller 测试与 `go test ./...` 通过，`git diff --check` 通过。
- 用户明确禁止 antigravity/Claude，因此未调用外部模型审查。
