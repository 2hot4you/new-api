# Group Pricing Metadata and Order Design

## Goal

让管理员在现有分组定价页面统一配置分组展示顺序、LobeHub 图标与推荐指数，并让 API Key 创建/编辑和列表使用同一份展示元数据。

## Compatibility boundary

现有 `GroupRatio`、`TopupGroupRatio`、`UserUsableGroups`、`GroupGroupRatio` 与 `AutoGroups` 的 JSON 形状保持不变。新增配置只负责展示，不参与计费、权限判断或 Auto 路由顺序。

## Persistence

新增 `group_ratio_setting.group_metadata`，值为有序数组：

```json
[
  {"name":"default","icon":"OpenAI.Color","recommendation":5},
  {"name":"vip","icon":"DeepSeek.Color","recommendation":4}
]
```

`name` 必须非空且唯一，`icon` 是 LobeHub key，最大 128 字符；`recommendation` 为 0–5 整数，0 表示不展示。配置缺失时使用空数组，无需数据库字段迁移。

## Backend contract

`GET /api/user/self/groups` 保持 `data` 为对象，仅在每个分组信息中增加可选字段：

```json
{"ratio":1,"desc":"默认分组","icon":"OpenAI.Color","recommendation":5,"display_order":0}
```

前端必须按 `display_order` 显式排序。未配置元数据的组排在已配置项后，再以名称作稳定兜底。`auto` 仍是普通可展示选项，但其展示顺序不影响 `AutoGroups`。

## Admin UI

分组定价表最左侧永久显示可访问的拖拽柄，并提供键盘上下移动能力。新增 Icon 与推荐指数两列：Icon 输入使用 `getLobeIcon` 实时预览，不自动改写键名；推荐指数输入限制 0–5。拖动与字段编辑都只标记现有表单 dirty，使用现有“保存分组倍率”入口提交。

## API Key UI

分组下拉按后端 `display_order` 排序。选中项图标 18px、下拉项 20px、列表分组单元格 16px。推荐指数仅在下拉中以紧凑“推荐 N/5”徽标展示，0 或缺失不渲染。无效 icon 使用现有 LobeHub fallback，不中断页面。

## Testing

后端覆盖元数据校验、缺省兼容、排序 join、`auto` 与未配置组；前端覆盖拖拽序列化、Icon 原值与预览、推荐徽标、下拉排序、旧响应兼容和表格图标。最终运行相关 Go 测试、Web focused tests、typecheck、i18n、scoped lint/format 与差异检查。
