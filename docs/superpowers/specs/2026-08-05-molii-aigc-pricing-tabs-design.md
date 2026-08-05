# Molii AIGC Pricing 标签切换设计

## 目标

将 `/system-settings/billing/molii-aigc-video-pricing` 中上下同时展示的两个价格表单改为与 `model-pricing` 一致的顶部标签切换，并消除顶部同时出现两个“保存更改”按钮的问题。

## 交互

- 顶部标签依次为 `Seedance 2.0`、`Grok Imagine`。
- 默认打开 `Seedance 2.0`。
- 标签使用项目现有 `Tabs`、`TabsList`、`TabsTrigger`、`TabsContent` 组件。
- 标签切换不刷新页面，也不改变当前路由。
- 内容区只挂载当前标签对应的价格表单。
- 顶部只挂载当前表单的一个“保存更改”按钮；切换标签时按钮及其保存目标同步切换。

## 组件边界

- 新增一个仅负责标签状态、标签标题 Portal 和活动内容渲染的 AIGC 定价容器组件。
- `StarAIVideoPricingSection` 与 `MoliiGrokPricingSection` 继续分别管理自身表单状态、校验和保存请求。
- `section-registry.tsx` 仅负责准备两个表单的默认值并将其传给容器。
- 不修改后端配置键、保存 API、路由或定价计算逻辑。

## 未保存状态

由于内容区只挂载当前标签，切换标签会卸载当前表单。现有 `FormNavigationGuard` 继续保护离开页面行为；标签切换本身不会触发浏览器路由拦截。因此用户应先保存当前标签再切换，页面只提供当前活动表单的保存按钮。

## 测试与验证

- 测试默认标签为 `Seedance 2.0`。
- 测试默认仅渲染 Seedance 表单。
- 测试点击 `Grok Imagine` 后仅渲染 Grok 表单。
- 测试任一时刻只有一个活动表单，因此只会注册一个顶部保存操作。
- 运行目标前端测试、TypeScript 类型检查、格式检查和前端构建。

## 范围外

- 不调整价格字段、默认价格或币种。
- 不修改其它系统设置页面。
- 不增加 URL 查询参数或新路由。
