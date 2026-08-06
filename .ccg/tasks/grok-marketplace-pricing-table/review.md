# Grok 模型广场价格总览表审查

## 范围

- Grok 模型卡片价格摘要与三列价格矩阵。
- 图片、旧版视频、Video 1.5 三种定价结构。
- 中等桌面宽度下的卡片响应式排版。
- 中英文文案及可访问性表格语义。

## 审查结论

- Critical：无。
- Warning：无。
- Info：价格完全来自后端 `molii_grok_pricing`，前端没有复制后台默认价格；缺少输出档位时显示不可用状态。
- 浏览器验收时发现旧版视频媒体输入在三列卡片内被挤成竖排，已将三列断点调整到 `2xl`，中等桌面宽度使用两列，并复验通过。

## 验证

- `bun test src/features/pricing`：31 项通过。
- `pnpm typecheck`：通过。
- `pnpm format:check`：通过。
- 修改文件定向 `oxlint`：通过。
- `pnpm build`：通过。
- `go test ./...`：通过。
- `GET http://127.0.0.1:3000/api/status`：200。
- 浏览器 `/pricing`：页面、筛选交互、4 张 Grok 价格表、Seedance 回归和控制台检查均通过。

## 外部审查说明

按用户明确要求，本任务未调用 antigravity 或 Claude；由 Codex 完成代码自审、自动化测试与浏览器验收。
