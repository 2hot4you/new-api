# 变更审查

## 结论

- Critical：0
- Warning：0
- Info：1
- 结论：本次顶部字标改动可提交。

## 审查范围

- `web/public/molii-wordmark.png`
- `web/src/components/layout/components/header-brand.tsx`
- `web/src/components/layout/components/public-header.tsx`
- `web/src/components/layout/components/__tests__/header-brand.test.tsx`

## 检查结果

1. 默认 `/logo.png` 且无自定义 React Logo 时，只渲染 Molii 横向字标，不重复显示站点名称。
2. 自定义系统 Logo 与自定义 React Logo 分支仍使用原有方形 Logo + 站点名称结构。
3. 原方形 Logo、favicon、页脚、控制台和系统配置逻辑未改动。
4. 字标保留原始字形与配色，仅做裁白边、白底透明化和透明内边距处理。
5. 图片声明固有宽高，实际导航显示为 70 × 28 px，避免加载时布局跳动。
6. 桌面与 390 px 窄屏均完成本地页面核验，未发现品牌区与导航按钮重叠。

## 验证证据

- 新增定向测试：4 passed / 0 failed。
- 前端类型检查：通过。
- 变更文件定向 oxlint：通过。
- 全量格式检查：通过。
- `git diff --check`：通过。
- 本地服务：`127.0.0.1:3000` 与前端开发服务均可访问。

## 已知基线问题（不属于本次变更）

- `bun test` 全量运行会因仓库大量测试使用 `node:test` 而触发 Bun 的跨文件嵌套 `describe` 限制，并另有日志、兑换码旧用例失败；本次新增用例单文件运行稳定通过。
- 全量 oxlint 存在大量本次变更之外的既有规则错误；本次涉及文件的定向检查通过。
- 按用户要求未执行生产构建，也未调用 antigravity 或 Claude。
