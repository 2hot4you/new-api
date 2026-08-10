# Docusaurus Serif 字体设计

## 目标

让 Molii Docusaurus 文档站与 New API 当前的 Anthropic + Serif 默认主题使用同一套字体，同时保留 Docusaurus 的默认 MDX、Infima 布局、颜色和组件样式。

## 设计

- 使用 New API Serif 字体轴采用的 `@fontsource-variable/lora`，由文档站本地依赖提供字体文件，不依赖外部字体 CDN。
- 复用 New API 的 CJK Serif 回退栈，确保 Lora 没有覆盖的中文字符仍显示为衬线字体。
- 只增加一个字体样式入口，覆盖 Infima 的正文字体与标题字体变量。
- 不覆盖等宽字体，因此代码块、行内代码和终端示例继续使用 Docusaurus 默认等宽字体。
- 不新增颜色、间距、布局、圆角、阴影或组件样式。

## 验收

- 浏览器计算后的正文和标题字体均以 `Lora Variable` 开头。
- 普通 MDX 和 API 页面继续使用 Docusaurus 默认渲染器。
- 本地开发服务在 `127.0.0.1:3100` 正常运行。
- 只运行测试和本地开发服务，不执行静态构建或部署。
