# Molii 开发者文档

这是独立的 Docusaurus 静态文档应用，首版只提供简体中文内容。

## 本地开发

```bash
cp .env.example .env
bun install --frozen-lockfile
bun run dev
```

开发服务器固定运行在 `http://127.0.0.1:3100`。环境变量只允许公开的文档站点配置；不要在 `.env` 中放置密钥。

## 验证

```bash
bun test
bun run build
```

构建输出位于 `build/`。
