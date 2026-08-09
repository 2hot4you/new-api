# Molii Docusaurus 用户文档站设计

## 目标

在 New API 同一 Git 仓库中新增专业、可搜索、可自托管的 Molii 用户文档站。公开内容只包含普通用户平台操作文档和用户侧 API 文档，首版覆盖 Seedance、Grok Imagine、临时素材及关联用户功能。

## 仓库边界

采用“同仓库、独立应用”结构，在根目录新增 `docs-site/`：

```text
new-api/
├── docs/                         # 现有工程资料与原始规范，不直接发布
├── docs-site/                    # 独立 Docusaurus 应用
│   ├── package.json
│   ├── bun.lock
│   ├── docusaurus.config.ts
│   ├── sidebars.ts
│   ├── docs/
│   ├── openapi/
│   ├── scripts/
│   ├── src/
│   └── static/
├── web/                          # 不修改
└── Go 后端                       # 不耦合文档构建
```

`docs-site/` 有独立依赖、锁文件、开发服务器和静态构建产物。不创建根 JavaScript workspace，不修改现有 `web/` 构建，不把文档站编入 Go 二进制或 New API Docker 镜像。

## 运行与部署边界

开发阶段仅在本机常驻运行，使用不与 3000、8787、8788 冲突的端口。源码变更由 Docusaurus 开发服务器热更新。

生产阶段只输出标准静态目录，由用户自己的服务器和反向代理部署。仓库不添加 Cloudflare Pages、Vercel、GitHub Pages或其他第三方云部署配置。站点 URL、Base URL、文档端口均通过无密钥环境配置提供 `.env.example`。

## 内容范围

### 普通用户平台操作

- 注册、登录、退出与账户安全。
- 控制台、模型广场与 Playground。
- API Key 创建、限制、禁用和删除。
- 临时素材创建、状态、URI、过期和删除。
- 普通使用日志、图片生成记录、视频生成记录、结果预览与计费明细。
- 钱包、余额、充值、兑换码、订阅和账单只按实际启用功能展示。
- 个人资料、密码、登录设备、Passkey 和 2FA。

### 用户侧 API

- Base URL、Bearer API Key、请求/响应、安全和错误约定。
- 异步任务生命周期、轮询、幂等、重试与下载。
- Seedance 2.0 标准/Fast、多模态、临时素材、音频和工具参数。
- Grok Imagine 图片生成/编辑。
- Grok Imagine 视频生成、图生视频和受支持的视频编辑。
- Models、Images、Videos、Assets 的公开 API Reference。
- curl、Python 和 TypeScript 完整示例。

### 明确排除

- 管理员、渠道、用户管理、模型管理、系统设置和运维部署。
- PostgreSQL、Redis、Session、内部 Cookie、迁移和反向代理配置。
- 上游凭据、系统访问令牌、渠道 ID、上游管理域名、内部任务/素材 ID。
- 真实 API Key、客户素材、签名 URL 和内部价格锚点。
- 现有 `docs/openapi/api.json` 的管理接口。

## 信息架构

站点顶级导航为：首页、快速开始、平台使用、API 基础、模型指南、API Reference、示例与工具、帮助与更新日志。中文简体为默认语言；英文结构预留但首版不要求完整翻译。首版只维护 current，不创建版本快照。

## API 契约

现有 `docs/openapi/*.json` 仅作为审计输入，不能直接复制到公开站。文档站维护显式 public allowlist，只生成已实现且属于首版产品范围的端点。

公开 OpenAPI 必须包含稳定唯一的 `operationId`、生产/测试 server 占位、Bearer security、真实请求 DTO、条件必填/枚举/范围/默认值、成功与失败示例、错误码、轮询和计费提示。API Reference 由 Docusaurus OpenAPI 插件生成，叙述型指南使用人工维护 MDX。

公开构建必须物理排除管理 schema 和未使用组件，不能依赖隐藏侧栏。

## 安全

- 首版 API Reference 不提供浏览器内持久化 API Key。
- 示例统一使用 `$MOLII_API_KEY`、`$MOLII_BASE_URL`、`task_xxx` 和 `example.com`。
- 付费 POST 不自动重试；异步轮询有退避和总超时。
- 构建检查真实密钥模式、管理路径、上游品牌及内部域名。
- 平台截图只能使用普通用户界面，且必须脱敏。

## 本地开发与质量门禁

本地脚本提供开发、构建、类型检查、OpenAPI 校验、链接检查和禁词/密钥扫描。Docusaurus 开发服务器负责页面热更新；构建输出可以由任意静态 Web Server 验证。

验收包括：Bun 冻结安装、TypeScript 检查、OpenAPI lint、生成 API MDX、Docusaurus build、断链检查、移动端导航、代码复制、搜索、基础可访问性以及公开内容边界扫描。

## 非目标

- 首版不覆盖通用文本、语音、Embedding 等全部 New API `/v1` 接口。
- 不构建管理员文档站。
- 不配置第三方云托管。
- 不把旧 PDF 作为内容源；PDF 仅可由当前文档站另行生成。
