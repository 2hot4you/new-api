# 恢复审查

## 范围

- `web/src/features/playground/` 已逐字恢复至 `d2f3dad0`（第一笔 Playground 改造提交的父提交）。
- 删除本轮新增的 Base 布局、参数默认值、存储迁移与组件测试文件。
- 七个语言资源仅删除新版专用的 `New conversation` 文案。
- Docusaurus 的 Playground 操作说明恢复至 `88b511e1^`，没有修改其他文档页面。
- Grok、COS、用户文件、认证、日志、本地 3000 入口均未修改。

## 验证

- 精确基线检查：Playground 源码与改造前基线无差异。
- Web：`bun run typecheck && bun test`，251 passed、0 failed。
- Docs：`bun test`，92 passed、0 failed。
- `git diff --check`：通过。
- 本地开发环境重启后 `/playground` 与对应文档页均返回 HTTP 200。

依据用户约束，未调用 antigravity、Claude 或子代理。
