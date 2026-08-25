# Requirements

- Production 文档继续允许公共搜索引擎收录。
- Development 文档继续输出 `noindex, nofollow`，避免与正式站重复收录。
- Development 与 Production 构建都必须生成 `${DOCS_BASE_URL}sitemap.xml`。
- Sitemap 只列出实际、公开的文档 HTML 路由，不列出静态资源、调试路由或 404。
- Sitemap URL 必须使用对应环境的 `DOCS_SITE_URL` 与 `DOCS_BASE_URL`。
- 不在静态产物中暴露任何 Secret。
- Algolia Development Crawler 能以 Development Sitemap 作为页面发现入口。
