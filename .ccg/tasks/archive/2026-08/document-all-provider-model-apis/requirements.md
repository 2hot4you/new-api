# 需求草案

- 在现有 `docs-site/` Docusaurus 文档系统中，以 Provider 为主分类。
- 每个 Provider 分类下覆盖当前公开的全部模型 ID。
- 每个模型页面提供与其实际端点和能力一致的调用文档。
- 保持 Docusaurus 默认 MDX 风格，不引入独立视觉体系。
- 不公开管理员配置、渠道密钥、内部路由策略或其他敏感信息。
- Provider、模型及排序以 Development 的公开 `https://dev.molii.co/api/pricing` 为权威数据源。
- 当前基线为 10 个 Provider、35 个模型。
- 文档生成不得在普通 `dev`/`build` 阶段依赖 Development 网络可用性。
