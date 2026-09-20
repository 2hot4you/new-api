# Molii 开发者文档

这是独立的 Docusaurus 静态文档应用，首版只提供简体中文内容。

## 本地开发

```bash
cp .env.example .env
bun install --frozen-lockfile
bun run dev
```

开发服务器固定运行在 `http://127.0.0.1:3100`。环境变量只允许公开的文档站点配置；不要在 `.env` 中放置密钥。

### 后台开发服务

仓库提供 `scripts/watch-and-run.sh` 作为本地开发启动入口。它直接运行 Docusaurus 开发服务器，
只监听本地源码并使用热更新，不执行生产构建、静态部署或 Docker 打包。

查看服务状态：

```bash
curl --fail http://127.0.0.1:3100/
```

## 验证

```bash
bun run check
```

`check` 会运行文档测试、公开内容与密钥检查、生产构建，以及不访问互联网的内部链接检查。外部链接检查可能受网络状态影响，因此按需单独运行：

```bash
bun run check:links:external
```

## 本地搜索

站点使用中文本地搜索（`@node-rs/jieba` 分词）。搜索索引只在生产构建中生成；先运行 `bun run build`，再运行 `bun run preview`，然后在 `http://127.0.0.1:3100` 测试搜索。开发服务器不会生成搜索索引。

## 静态自托管

构建输出位于 `build/`，可由任意静态文件服务托管。Nginx 的最小静态示例见 [`examples/nginx.conf.example`](examples/nginx.conf.example)；将其中的 `root` 路径替换为实际的 `build/` 目录即可。该示例只处理静态文件和缓存，不包含部署自动化、上传或凭据配置。

### 同域 `/docs/` 部署

Molii 的正式与开发环境将文档作为主站下的静态目录发布，不需要在服务器运行 Docusaurus 进程：

```bash
DOCS_ENV=development \
DOCS_SITE_URL=https://dev.molii.co \
DOCS_BASE_URL=/docs/ \
DOCS_API_BASE_URL=https://dev.molii.co \
bun run build
```

把 `build/` 的内容发布到主站静态根目录下的 `docs/`。OpenResty 应将 `/docs` 和 `/docs/` 重定向到 `/docs/quick-start`，并直接提供 `/docs/assets/` 与其他生成文件。生产环境把两个 `dev.molii.co` 值替换为 `molii.co`，并使用 `DOCS_ENV=production`。


### Claudeye 国内站与海外站

两站共用文档源码和 `claudeye` 品牌，文档与 API 均使用各自域名。
文档发布不启动或重启应用、PostgreSQL、Redis，也不覆盖整个 1Panel 站点配置。

| GitHub Environment / 手动 target | DOCS_SITE_URL 与 DOCS_API_BASE_URL | 主机上的公开目录 |
| --- | --- | --- |
| production-claudeye | https://claudeye.com | /opt/1panel/www/sites/claudeye.com/index/docs |
| production-model-claudeye | https://model.claudeye.com | /opt/1panel/www/sites/model.claudeye.com/index/docs |

每个 Environment 配置以下 Variables（均为公开配置）：

| Variable | 值 |
| --- | --- |
| DEPLOY_DIR | /opt/claudeye/production（沿用应用配置） |
| DOCS_ENV | production |
| DOCS_SITE_URL / DOCS_API_BASE_URL | 上表对应域名，不带 /docs |
| DOCS_BRAND_ID | claudeye |
| DOCS_SITE_TITLE | claudeye 开发者文档 |
| DOCS_NAVBAR_TITLE | claudeye |
| DOCS_TAGLINE | claudeye AI API 开发指南 |
| DOCS_DEFAULT_FONT | sans |
| DOCS_LOGO_PATH | img/brand/logo.svg |
| DOCS_FAVICON_PATH | img/brand/favicon.png |
| DOCS_SOCIAL_IMAGE_PATH | img/brand/social.png |
| DOCS_LOGO_SOURCE_URL | 对应本站域名 + /claudeye-placeholder.svg |
| DOCS_FAVICON_SOURCE_URL | 对应本站域名 + /claudeye-placeholder-32.png |
| DOCS_SOCIAL_IMAGE_SOURCE_URL | 对应本站域名 + /claudeye-placeholder-180.png |

占位图片沿用已发布应用的公开资源。先确认这三个 URL 返回正确图片（非 SPA HTML），
再触发文档部署。文档下载器要求 HTTPS、图片 Content-Type，拒绝跳转。
SSH 与 Telegram Secrets 沿用该 Environment；不添加数据库或业务密钥。

#### 服务器首次准备（每台分别执行）

本次只读检查确认两台 OpenResty 均将 `/opt/1panel/www` 挂载为 `/www`，
两站 `index` 目录已存在、root:root 755，`index/docs` 尚不存在。
因此容器内静态 root 分别为 `/www/sites/claudeye.com/index` 与
`/www/sites/model.claudeye.com/index`；实施时应再次核对挂载。
公开 `index/docs` 目录应由 `claudeye-deploy` 可写，OpenResty worker 可读且所有父目录可遍历。
仅调整 docs 目录，不能把整个 `/opt/claudeye/production` 开放给 Web 服务器，不能暴露
`.env.runtime`、data、日志、备份或凭据。发布快照仍存放在私有的
`/opt/claudeye/production/data/docs-deploy`。

在已有站点 `server` 块中增加 `examples/nginx.conf.example` 的四个 `/docs` location，
每个静态 location 的 `root` 指向 **OpenResty 容器内**该站的 `index` 目录。
保留现有 `/` 反代、TLS、证书、其他站点及洛杉矶的 23000 服务。
先备份实际修改文件，确认无重复 location，再执行 `openresty -t`，成功后 reload。
校验失败恢复该文件，不 reload；公网异常恢复旧配置并重新检查、reload。

`/docs`、`/docs/` 应返回 308 到 `/docs/quick-start`；发布后的
`/docs/quick-start/` 返回包含 claudeye 标题的 HTML，静态资源正常，未知文件返回 404。
上线前空目录尚无正文，不能把这种状态当成文档验收成功。

#### 发布及验收

在 **Build and deploy documentation** 中 Run workflow，Branch 选择 `main`，
一次选择一个上表 target。没有 source_ref 参数，构建对应工作流触发 SHA。
海外站已通过首次文档发布验收；main 自动矩阵与文档 `all-production` 包含
molii、ixiaozu、production-model-claudeye。上海 production-claudeye 仍需单独手动触发。
自动 push 只在 `docs-site/**` 或 `.github/workflows/docs-deploy.yml` 变化时触发，
develop 仍只发布开发文档。应用工作流的自动矩阵与 all-production 范围不受影响。

首次先在 develop 完成验证，再按授权合并 main。注意文档变更合并 main
会自动发布上述三个生产文档；上海文档继续通过单独 target 手动发布。

验收各自 `/docs` 入口、导航、图片、API 地址、控制台链接和本地搜索。
公开页面校验失败时发布脚本恢复上一文档快照，首次无旧文档时恢复空目录；查看 Actions 与 Telegram 结果。
不要用应用部署按钮代替文档发布。
