# Molii Wordmark 全局统一设计

## 目标

默认 Molii 品牌下，公共 Header、首页 Footer 与登录后的控制台品牌入口统一使用同一张横向 `/molii-wordmark.png`，不再显示旧的圆角 `/logo.png` 与重复的 `Molii` 文字组合。

## 根因

`HeaderBrand` 对默认 `DEFAULT_LOGO` 有专门分支，会渲染 `/molii-wordmark.png`；`HomeFooterContent` 和 `SystemBrand` 则直接渲染系统 Logo，因此默认配置仍显示 `/logo.png`。品牌判断和视觉资源分散在不同组件中，导致界面不一致。

## 组件设计

新增一个布局层共享的 `MoliiWordmark` 展示组件，集中维护：

- 图片路径 `/molii-wordmark.png`。
- 固定的原始宽高比 `375 / 150`。
- `data-molii-wordmark` 测试标记。
- 可由调用方传入的尺寸与交互样式。

`HeaderBrand`、`HomeFooterContent` 和 `SystemBrand` 只负责决定当前是否属于默认 Molii 品牌：系统 Logo 等于 `DEFAULT_LOGO` 且没有自定义 React Logo 时使用共享 Wordmark，否则继续使用现有自定义 Logo 与站点名。

## 尺寸

- 公共 Header：保持现有约 `28px` 高度。
- 控制台顶栏：使用紧凑横向 Wordmark，约 `24px` 高，替换原 `20px` 圆角图标与文字。
- 首页 Footer：使用约 `48px` 高的横向 Wordmark，移除旁边重复的彩色 `Molii` 文本。

## 兼容性

- 自定义站点 Logo 或自定义站点名不被强制替换，仍显示自定义 Logo 与名称。
- 控制台 Sidebar 变体保持现状；本次只处理用户指出的 `variant='inline'` 顶栏入口。
- 自定义 Footer HTML 路径保持现状。
- 不修改图片文件，不新增依赖，不运行生产构建。

## 测试

- Header 默认品牌仍使用共享 Wordmark，且不重复站点名。
- Footer 默认品牌使用共享 Wordmark，尺寸更大且不再输出旧 `/logo.png`。
- 控制台 inline 品牌默认使用共享 Wordmark，不再输出旧圆角 Logo 与重复名称。
- 三处的自定义品牌回退行为保持不变。
