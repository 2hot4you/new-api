# Molii Favicon 实施审查

## 结论

- Critical：0
- Warning：0（本次变更范围）
- Info：1（仓库全量前端测试存在既有隔离问题）

## 实现核对

- 新增透明背景粉色 `m` SVG favicon。
- 新增 32×32 PNG 与 180×180 Apple Touch Icon。
- 旧 `favicon.ico` 已替换为嵌入同一 32×32 PNG 的单图标 ICO，Rsbuild 自动注入路径不会再显示旧圆形 Logo。
- HTML 显式声明 SVG、PNG 与 Apple 图标，并使用 `?v=1` 缓存版本。
- 默认 `/logo.png` 在运行时会映射到独立 Molii favicon。
- 自定义品牌 Logo URL 保持原样。
- 无新增依赖、无密钥、无生产构建。

## TDD 证据

- `dom-utils.test.ts` 首次运行因 `MOLII_FAVICON_URL` 缺失而失败，完成实现后 3/3 通过。
- `favicon-assets.test.ts` 首次运行因声明和资源缺失而 0/2 失败，新增资源后通过。
- ICO 回归断言首次发现旧文件包含 3 个图标而失败，替换后 3/3 通过。

## 最终验证

- 定向品牌与 favicon 测试：19 通过，0 失败。
- TypeScript：通过。
- 定向 Oxlint：通过。
- 全项目格式检查：通过（1173 files）。
- `git diff --check`：通过。
- 本地 `http://127.0.0.1:3000/`：页面标题为 `Molii`，运行时仅声明新的 SVG/PNG/ICO favicon 与 Apple Touch Icon；资源均返回 200。
- 本地后端 3000 与前端内部热更新 3001 正常运行。

## 已知仓库基线问题

`bun test` 全量执行得到 133 通过、67 失败、59 errors；`--max-concurrency=1` 结果相同。主要错误是 Bun 对多文件共享 `node:test`/DOM 状态产生 `describe() inside another test()`，另有既有日志与兑换码 UI 用例失败。失败文件与本次 favicon 变更无关；本次 19 项定向回归、类型、lint 和格式检查均独立通过，因此未扩大范围修复全量测试基础设施。

按用户明确要求，未调用 antigravity 或 Claude；本次采用本地自审。
