# Requirements

- `POST /v1/videos` 的 Doubao `openai_video` 端点选择类型 61 StarAI 渠道后，渠道身份校验必须通过。
- 类型 61 必须继续使用原生 StarAI 适配器；54/45 继续使用 Doubao 插件。
- 不得允许无关原生渠道绕过插件身份约束。
- 渠道上下文初始化失败时必须中止请求，不能带着空 `channel_type` 或 `base_url` 继续执行。
- 添加覆盖真实 pinned endpoint、身份过滤及 SetupContext 的回归测试，不调用付费上游。
