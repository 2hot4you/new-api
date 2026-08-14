# 审查结果

## 结论

- Critical：0
- Warning：0
- Info：0

## 根因与修复

- 根因：Header、Footer 与控制台分别维护默认 Logo 渲染逻辑，只有 Header 将 `/logo.png` 转换为 `/molii-wordmark.png`。
- 修复：新增共享 `MoliiWordmark` 组件，三处默认品牌共用同一图片来源与宽高比。
- Footer 使用 `120 × 48` 大号 Wordmark；控制台顶栏使用 `60 × 24` 紧凑 Wordmark；Header 保持原尺寸。
- 自定义 Logo、站点名、Sidebar 品牌和自定义 Footer HTML 均保持原路径。

## 验证

- TDD RED：Header 缺共享标记、Footer 仍输出 `/logo.png`、控制台纯展示组件缺失，三项均按预期失败。
- TDD GREEN：四个相关测试文件合计 `24 pass / 0 fail`。
- TypeScript、定向 Oxlint、全量格式检查和 `git diff --check` 通过。
- Chrome 本地开发页实测：控制台顶栏仅有一个 `/molii-wordmark.png`，尺寸 `60 × 24`；Footer 仅有一个 `/molii-wordmark.png`，尺寸 `120 × 48`；两处均无旧 `/logo.png`、重复品牌名或横向溢出。
- 页面日志只有浏览器扩展自身的版本提示，与应用无关。
- 未运行生产构建，未调用外部模型。
