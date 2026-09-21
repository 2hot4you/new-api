# claudeye 运行时品牌配色设计

## 1. 背景与目标

`claudeye.com` 与 `model.claudeye.com` 使用同一套应用代码，但拥有独立 PostgreSQL、Redis、用户体系、渠道与站点配置。两站当前通过 `VITE_SITE_PROFILE=claudeye` 和 `DOCS_BRAND_ID=claudeye` 选择品牌构建，但 Logo 与 Favicon 仍是静态占位素材。

本设计将用户提供的九宫格图形与 `claudeye` 字标接入应用首页和文档站，并允许超级管理员在每个环境中独立配置浅色、深色场景的图形与字标颜色。保存后，新颜色不依赖重新构建或重新部署即可生效。

## 2. 已选方案

采用“数据库配置 + 后端动态 SVG”方案。

未采用的方案：

- 前端拉取颜色并使用 CSS 重新绘制：应用和 Docusaurus 需要维护两套渲染逻辑，容易产生首屏闪烁，动态 Favicon 也更复杂。
- 保存配置时写入静态文件：需要处理容器文件系统、权限、双服务器同步和缓存失效，不适合当前不可变部署流程。

动态 SVG 由后端使用固定、安全的品牌几何结构和严格校验后的颜色生成。应用站和文档站只引用资源地址，因此共享同一渲染结果。

## 3. 管理员配置

### 3.1 配置入口

在 `系统设置 → 站点设置` 中新增“品牌外观”分区。该分区仅在前端构建品牌为 `claudeye` 时显示，并继续复用现有超级管理员路由与 `/api/option/` 权限校验。

页面包含四个字段：

| 配置键 | 后台标签 | 默认值 |
| --- | --- | --- |
| `brand_setting.claudeye_light_mark_color` | 浅色区域图形颜色 | `#242424` |
| `brand_setting.claudeye_light_text_color` | 浅色区域字体颜色 | `#6A6A6A` |
| `brand_setting.claudeye_dark_mark_color` | 深色区域图形颜色 | `#FFFFFF` |
| `brand_setting.claudeye_dark_text_color` | 深色区域字体颜色 | `#B8B8B8` |

每个字段由原生颜色选择器、可编辑 HEX 输入框和当前色块组成。输入只接受完整的 `#RRGGBB`，保存前转换为大写。表单提供：

- 浅色 Header 实时预览；
- 深色 Footer 实时预览；
- 对比度提示；
- “恢复默认配色”操作。

对比度不足只警告，不阻止保存，以保留管理员的最终控制权。

### 3.2 保存与隔离

四个值沿用现有 options 表存储。两个生产环境连接不同数据库，因此天然隔离，不增加站点 ID 列，也不跨环境同步。

后端在默认 OptionMap 中注册四个中性灰色默认值。首次部署不需要数据库迁移；管理员首次保存时创建或更新对应 option 行。

后端对四个键执行同样的 `#RRGGBB` 校验。前端校验只改善交互，不能替代后端校验。

## 4. 动态品牌资源接口

新增公开只读资源：

- `GET /api/branding/claudeye/wordmark.svg?surface=light`
- `GET /api/branding/claudeye/wordmark.svg?surface=dark`
- `GET /api/branding/claudeye/favicon.svg`

字标接口根据 `surface` 选择对应的图形色和字体色。Favicon SVG 使用透明背景，并通过 SVG 内部的 `prefers-color-scheme`：

- 浏览器浅色主题使用浅色区域图形颜色；
- 浏览器深色主题使用深色区域图形颜色。

为了让未保存表单也能复用同一渲染器，字标接口允许可选的 `mark` 和 `text` 查询参数作为预览覆盖值。覆盖值同样必须通过严格颜色校验，仅影响本次响应，不写入数据库。

### 4.1 SVG 构成

- 九宫格图形使用固定 SVG 矩形和旋转几何，不接受用户路径输入。
- `claudeye` 字标使用用户提供素材提取出的本地 alpha mask，通过 Go `embed` 嵌入二进制，不请求外部资源。
- 颜色只写入经过验证的 `fill` 属性。
- SVG 不包含脚本、事件属性、`foreignObject` 或远程 URL。

### 4.2 响应与缓存

成功响应设置：

- `Content-Type: image/svg+xml; charset=utf-8`；
- `X-Content-Type-Options: nosniff`；
- `Cache-Control: no-cache, must-revalidate`；
- 根据当前四个颜色生成稳定 ETag，允许浏览器协商缓存。

这能避免长期缓存旧配色，同时不要求每次刷新都重新传输完整 SVG。无效 `surface` 或颜色覆盖返回 HTTP 400。

## 5. 应用站接入

### 5.1 首页 Header

当 `SITE_BRAND.id === 'claudeye'` 且后台未设置不同于默认值的自定义 Logo URL 时，`HeaderBrand` 使用浅色动态字标，并不再在右侧重复渲染系统名称。

如果超级管理员已配置自定义 Logo URL，则保留现有“Logo + 可配置系统名称”行为。该规则让新品牌字标成为默认表现，同时不破坏已有覆盖能力。

### 5.2 首页 Footer

默认 claudeye 品牌使用深色动态字标。自定义 Logo URL 仍沿用现有 Footer 行为。

### 5.3 Favicon

claudeye 构建默认 Favicon 地址改为动态 Favicon 接口。系统配置加载默认 Logo 时，不得再用横向 Logo 覆盖专用 Favicon；只有明确配置的自定义 Logo 才沿用现有动态覆盖行为。

应用保留静态中性灰色 Favicon 和字标作为加载失败回退。

## 6. 文档站接入

文档站与 API 位于同一域名，因此可以直接引用该环境的动态品牌资源。

### 6.1 导航栏

claudeye 文档导航栏使用绝对同源地址 `/api/branding/claudeye/wordmark.svg?surface=light`。完整字标已包含品牌文字，因此不再额外显示 Docusaurus `navbar.title`。

### 6.2 Footer

自定义文档 Footer 使用 `surface=dark` 的动态字标。加载失败时回退到构建时准备的中性灰色品牌素材。

### 6.3 Favicon

Docusaurus HTML 为 claudeye 注入动态 SVG Favicon link，同时保留现有静态 Favicon 作为回退。Molii 和 iXiaozu 继续使用原有静态配置。

文档社交分享图不在本次范围内，继续使用现有静态图片，避免社交抓取机器人对动态资源支持不一致。

## 7. 安全与权限

- 颜色写入继续走只允许超级管理员访问的 option 接口。
- 动态 SVG 接口公开可读，但只能读取四个非敏感颜色配置。
- 后端只接受固定键和 `#RRGGBB`，不拼接任意 CSS、HTML、路径或 URL。
- 资源响应禁止 MIME 猜测，不引用外部资源。
- 管理员页面不向普通用户暴露 option 列表或编辑能力。

## 8. 错误处理与兼容性

- 数据库中出现非法历史值时，渲染器记录错误并回退到对应默认色，不输出非法 SVG。
- 动态品牌接口不可用时，页面其他功能继续工作；应用和文档使用静态中性灰色回退素材。
- 旧浏览器不支持 SVG Favicon 时继续使用 PNG 回退。
- 现有 Logo URL、System Name、Footer 自定义内容保持兼容。
- 非 claudeye 构建不显示新设置分区，也不改变 Header、Footer 或文档品牌行为。

## 9. 测试策略

### 9.1 后端

- 四个默认 option 注册正确；
- 合法颜色标准化并保存；
- 非法颜色通过 API 和 model 层都被拒绝；
- light/dark 字标选择正确颜色；
- Favicon 包含明暗主题颜色；
- 查询覆盖不写入数据库；
- 响应 Content-Type、安全头、缓存头和 ETag 正确；
- 输出不包含脚本、事件属性或远程 URL。

### 9.2 应用前端

- 仅 claudeye 显示品牌外观设置；
- 四个选色器、HEX 输入、恢复默认、校验和双场景预览正确；
- claudeye 默认 Header 不重复系统名称；
- Header 与 Footer 分别使用 light/dark 动态字标；
- 自定义 Logo URL 继续覆盖默认字标；
- 默认 Logo 加载不会覆盖专用 Favicon。

### 9.3 文档站

- claudeye 导航栏不重复标题；
- Navbar、Footer 和 Favicon 使用同域名动态资源；
- 静态回退存在且路径有效；
- Molii 与 iXiaozu 配置保持不变；
- 两个 claudeye 域名均生成自己的同源动态资源地址。

### 9.4 构建与视觉检查

- Go 单元测试、受影响的前端 Vitest、前端类型检查与生产构建；
- Docusaurus 配置测试、类型检查与生产构建；
- 在浅色 Header、深色 Footer、移动端 Header、文档 Navbar 和浏览器标签页检查实际显示；
- 分别验证两套生产环境配置互不影响。

## 10. 发布与回滚

代码先进入 develop 验证，确认管理员保存、首页、文档和 Favicon 行为后再合并 main。两个 claudeye 生产环境部署同一版本，但保留各自数据库中的四个颜色值。

该功能不增加数据库 schema。回滚旧版本后，新 option 行会被旧版本忽略；恢复新版时继续生效。静态中性灰色素材始终保留，便于动态资源故障时回退。
