# Molii Favicon 品牌图标设计

## 目标

为 Molii 提供独立、清晰的浏览器标签页图标，不再直接缩小横向字标，也不再被后台系统 Logo `/logo.png` 动态覆盖。

## 现状与根因

- `web/index.html` 当前把 `/logo.png` 直接声明为 favicon。
- 应用启动后，`applyFaviconToDom` 又会把后台状态中的系统 Logo 设置成 favicon。
- 默认 `/logo.png` 是旧的圆形图标；横向 `/molii-wordmark.png` 在 16×16 或 32×32 场景中也不适合作为 favicon。

因此，favicon 必须从展示型 Logo 中拆分，成为独立品牌资源。

## 视觉方案

- 使用 Molii 字标中的粉色小写 `m` 作为核心图形。
- 使用透明背景和紧凑安全边距，确保在 16×16、32×32 与高分屏标签页中仍然可辨识。
- 不使用完整 `molii` 横向字标，不复用旧圆形 `/logo.png`。
- 颜色保持现有 Molii 粉蓝品牌体系，favicon 本体以粉色 `m` 为主。

## 资源与接入

- 新增浏览器优先使用的 SVG favicon。
- 提供 32×32 PNG 作为兼容回退。
- 提供 180×180 Apple Touch Icon。
- `web/index.html` 显式声明各图标资源，并附加版本参数避免旧缓存长期残留。
- 默认 Molii 品牌下，运行时 favicon 固定使用独立 favicon，不再从系统 Logo 推导。
- 非默认、自定义品牌仍可继续将自定义系统 Logo 用作运行时 favicon，保留 New API 的白标兼容能力。

## 缓存与兼容性

- favicon URL 使用版本参数，强制浏览器发现新资源。
- 不删除 `/logo.png`，避免破坏仍依赖默认系统 Logo 常量或自定义品牌回退的代码。
- Header、Footer、控制台继续使用 `/molii-wordmark.png`，不受 favicon 变更影响。
- 不增加运行时依赖，不执行生产构建。

## 测试与验收

- HTML 同时声明 SVG favicon、PNG fallback 与 Apple Touch Icon。
- 默认 `/logo.png` 不会再次覆盖 Molii favicon。
- 自定义外部 Logo 仍可被应用为自定义品牌 favicon。
- 资源尺寸、透明背景和引用路径正确。
- 相关单元测试、类型检查、代码检查和格式检查通过。
- 本地开发页面刷新后，浏览器标签页显示新的 Molii `m` 图标。
