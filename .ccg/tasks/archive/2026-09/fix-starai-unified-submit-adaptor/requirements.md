# Requirements

- `POST /v1/videos` 选择 StarAI（channel type 61）后必须使用原生 StarAI 请求构建器。
- 不能调用 Doubao JS 插件的 Ark URL 构建逻辑，也不能返回 `plugin_request_invalid`。
- Doubao/VolcEngine 类型 54/45 继续使用 Doubao 插件。
- 使用完整路由或提交编排回归测试覆盖渠道选择后的真实适配器行为。
- 不执行真实付费生成请求。
- 旧渠道或复制渠道的 `base_url` 为数据库 `NULL` 时，具备内置默认地址的渠道必须回退到该绝对地址，不能让插件构造相对 URL。
- 没有内置默认地址的自定义插件渠道仍返回空地址，继续要求管理员显式配置。
