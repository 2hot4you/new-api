# Docusaurus 同域 `/docs/` 双环境部署设计

## 背景与目标

Molii 的 Docusaurus 文档站已经位于仓库的 `docs-site/`，当前可以在本地 `127.0.0.1:3100` 开发，但正式服务器不应常驻文档进程。目标是把它作为纯静态资源发布到现有站点：

| 分支 | 环境 | 文档地址 | 主站静态目录 |
| --- | --- | --- | --- |
| `develop` | development | `https://dev.molii.co/docs/quick-start` | `/opt/1panel/www/sites/dev.molii.co/index/docs` |
| `main` | production | `https://molii.co/docs/quick-start` | `/opt/1panel/www/sites/molii.co/index/docs` |

`/docs` 和 `/docs/` 由用户已经保存的 OpenResty 配置永久重定向到 `/docs/quick-start`。本设计不新增 `docs.molii.co`、`dev-docs.molii.co`、Docker 容器或公网监听端口，也不登录服务器修改 1Panel/OpenResty。

## 方案选择

### 采用：独立文档工作流 + 静态目录发布

新增独立的文档 GitHub Actions 工作流。工作流根据分支为 Docusaurus 注入公开环境配置，构建 `docs-site/build/`，验证 `/docs/` 子路径，随后通过现有 SSH 凭据把压缩产物交给服务器侧部署脚本。脚本将产物同步到对应站点的 `index/docs`，执行公网健康检查，失败时恢复发布前快照。

该方案满足同域访问，不与 new-api 的 3000/3010 端口或本地 Docusaurus 3100 端口耦合。文档发布失败不会重启、回滚或修改 new-api 容器。

### 不采用：服务器常驻 Docusaurus

该方案需要新增 3100/3110 进程、反向代理和进程守护；静态文档没有必要承担这些运行成本与故障面。

### 不采用：把文档打进 new-api Docker 镜像

这会让每次文档修改都重建后端镜像，并把文档发布和应用数据库迁移、容器健康检查绑定在一起，不符合职责隔离目标。

## 构建配置

工作流只向静态构建注入以下公开变量：

| 环境 | `DOCS_ENV` | `DOCS_SITE_URL` | `DOCS_BASE_URL` | `DOCS_API_BASE_URL` |
| --- | --- | --- | --- | --- |
| development | `development` | `https://dev.molii.co` | `/docs/` | `https://dev.molii.co` |
| production | `production` | `https://molii.co` | `/docs/` | `https://molii.co` |

Development 继续输出 `noindex`，Production 允许搜索引擎索引。构建产物不得读取或包含 API Key、数据库/Redis 凭据、SSH 私钥、Telegram Token 或服务器 `.env.runtime`。

Docusaurus 的导航、Markdown 内链、首页链接、静态资源和本地搜索索引都必须在 `/docs/` 下工作。验证不能只在默认根路径 `/` 构建；必须额外以 `DOCS_BASE_URL=/docs/` 构建并爬取实际子路径。`/docs` 的重定向由 OpenResty负责，不由 Docusaurus 生成第二个落地页。

## GitHub Actions 拆分

新增 `.github/workflows/docs-deploy.yml`，仅在以下情况触发：

- `main` 或 `develop` 上的 `docs-site/**` 变更；
- 文档部署工作流自身变更；
- 手动 `workflow_dispatch`。

工作流包含四个职责清晰的 job：

1. **prepare**：把 `main`/`develop` 映射为环境、站点 URL、API URL、远端部署目录和公开文档目录。
2. **verify-and-build**：安装 Node 22 与 Bun 1.3.14，使用 frozen lockfile；分别运行非浏览器测试和浏览器测试，执行 forbidden/secret/API/catalog 检查；使用 `/docs/` 配置构建并进行内部链接爬取；把 `build/` 打成不含父目录的 tar.gz 并上传为 Actions artifact。
3. **deploy**：复用现有 SSH secrets，将唯一命名的构建包与部署脚本上传到 `/opt/molii/<environment>`，校验 SHA-256 后执行部署。
4. **notify**：无论成功、失败或取消都复用现有 Telegram secrets 通知，内容只包含环境、分支、短提交号和 Actions 链接。

文档工作流使用独立的 `docs-deploy-<environment>` concurrency group，不能取消正在进行的发布。现有后端 `.github/workflows/deploy.yml` 增加文档专属路径忽略规则，使纯 `docs-site/**` 修改不再重建 new-api 镜像；如果同一提交同时修改应用代码，应用和文档工作流仍分别执行。

## 服务器发布流程

新增 `docs-site/deploy/deploy.sh`，接受环境、发布 ID、构建包、校验值和健康地址。脚本只允许两个固定环境映射，不接受任意公网目录参数：

- production → `/opt/1panel/www/sites/molii.co/index/docs`
- development → `/opt/1panel/www/sites/dev.molii.co/index/docs`

脚本执行以下步骤：

1. 使用 `flock` 获取环境独立锁。
2. 验证构建包 SHA-256、tar 路径安全以及 `quick-start/index.html` 等必要文件。
3. 在私有 `/opt/molii/<environment>/data/docs-deploy/` 下解压 staging，并对当前公开目录创建回滚快照；不会复制或读取该环境的 `.env.runtime`。
4. 使用 `rsync --archive --delete --delay-updates` 将 staging 发布到公开 `docs/` 目录。公开目录只保存当前静态产物，不保存构建包、锁、日志或回滚包。
5. 验证 `/docs`/`/docs/` 的重定向目标和 `/docs/quick-start` 的 HTTP 200、页面标识及关键静态资源。
6. 任一发布或健康检查失败时，将快照同步回公开目录并保持脚本失败状态；首次发布无快照时清空失败的半成品。
7. 成功后删除临时构建包和 staging，只保留一份上一版本回滚快照。

该发布方式避免为静态文档新增运行时服务。`rsync --delay-updates` 将文件更新延迟到传输末尾，并在末尾删除旧资源；它不是目录级单一原子重命名，但混合版本窗口很短，并由构建哈希资源、健康检查和自动回滚共同控制。若未来要求严格目录级原子切换，可把 OpenResty alias 改到 `docs/current/` 并升级为 release symlink，本轮不要求用户再次修改已保存的配置。

## 一次性服务器前置条件

在首次 Actions 发布前，管理员需要确认：

- `molii-deploy` 可以写入两个站点的 `index/docs` 目录，但不获得整个站点根目录或 OpenResty配置目录的写权限；
- 服务器安装 `tar`、`sha256sum`、`rsync`、`curl` 和 `flock`；
- OpenResty 容器映射能够看到宿主机站点目录；
- 已保存的 `/docs` location 优先级高于主站反向代理 include。

如果目录不存在，应由管理员以 root 只创建并授权这两个具体目录。GitHub Actions 不执行 sudo、不放宽 `/opt/molii` 的秘密文件权限，也不调用 1Panel API。

## 主站文档入口

主导航在没有后台 `docs_link` 时已经回退到 `/docs`。部署后管理员应把系统设置中的文档链接设为 `/docs`，使主导航、首页按钮和页脚都使用同域文档；这是运行时配置，不硬编码生产域名。前端的内置兜底链接应统一为 `/docs`，避免全新安装仍跳到旧外部文档站。

## 测试策略

### 配置和内容

- `DOCS_BASE_URL` 规范化与公开变量白名单测试。
- Production/Development 的 `siteUrl`、API URL、`noindex` 和 `/docs/` base URL 契约。
- 生成目录包含 quick-start、Provider/Model 页面、API 参考和搜索资产。
- 在 `/docs/` 预览地址执行内部链接与 fragment 爬取，防止资源或导航回到站点根路径。

### 部署脚本

使用临时目录、mock `curl`/`rsync` 测试：

- 拒绝未知环境、不安全 tar 和错误 checksum；
- development/production 映射正确；
- 成功发布、删除陈旧文件、保留必要静态资源；
- 健康检查失败时恢复原内容；
- 日志和参数不包含 Secret。

脚本还必须通过 `bash -n` 与 `shellcheck`（环境存在时）；工作流 YAML 通过 `actionlint`（环境存在时）。

### 发布验收

- Development 首先发布并验证 `https://dev.molii.co/docs/quick-start`。
- Production 仅在相同代码后续进入 `main` 时发布。
- 两个站点的 `/api/status` 和 new-api 容器在文档发布期间保持不变。
- GitHub Actions 与 Telegram 能明确区分“文档部署”和“应用部署”。

## 范围外事项

- 不创建独立文档域名。
- 不新增数据库迁移、Redis 数据、API Key 或 COS 配置。
- 不修改 1Panel/OpenResty 配置或直接登录服务器。
- 不自动推送 `main`，不触发生产部署。
- 不为静态文档增加 Docker、Compose 或常驻 3100 端口。

## 受限审查说明

CCG 要求在中等复杂度任务中调用 antigravity 与 Claude 做并行架构分析。本机缺少 `~/.claude/bin/codeagent-wrapper`，两个调用均以状态 127 失败；本设计因此依据仓库现有双环境部署契约、Docusaurus 配置和本地自审完成。实施完成后会再次尝试规定审查，并把限制及本地验证证据记录到任务审查文件。
