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
