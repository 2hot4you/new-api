# Requirements

- 完成 Molii Grok Imagine P0、P1、P2：正式视频生成别名、编辑、延长、多参考图与用户自有 Files API。
- 图片、视频结果和用户上传文件复用腾讯云 COS，按对象创建时间固定保留 24 小时。
- 不支持 `grok-imagine-video-1.5-preview`，不增加 Grok 或 Seedance 渠道切换设计。
- Playground 保留现有请求能力，采用 assistant-ui Base 风格布局，所有高级参数默认关闭。
- 用户文档和 API 参考使用 Docusaurus 默认 MDX 样式，不公开管理接口、渠道信息或真实凭据。
- 只运行本地开发环境和验证，不生成生产构建、Docker 镜像或云部署产物。
