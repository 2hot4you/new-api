# 根因分析

## 真实环境证据

- Development 汇总 Tooltip 的 key 列包含 `总计：`、模型 ID 和 `Others` 等行。
- 自定义装饰逻辑可以定位 shape 列和 key 行，但 VChart 为 shape 列写入了 `display:none`。
- VChart 的 DOM Tooltip 实现会在 `dimension.updateContent` 返回的所有条目都缺少 `hasShape/shapeType` 时隐藏整个 shape 列。
- 当前排行榜的 `dimension.updateContent` 会重建 `{ key, value }` 条目，因此不再携带上述 shape 元数据。

## 根因

上一轮装饰逻辑成功生成并插入了 Lobe 图标，但没有覆盖 VChart 对 shape 列设置的 `display:none`，所以汇总 Tooltip 中图标节点存在却不可见。既有测试只验证了图标节点和间距，没有模拟 VChart 隐藏整列的真实状态。

## 最小修复

- 统计本次装饰是否实际插入至少一个自定义图标。
- 仅在存在自定义图标时，把 shape 列恢复为 VChart 正常显示值 `inline-block`。
- `总计`、`Others` 等没有实体图标的纯汇总 Tooltip 保持隐藏，不制造空白图标列。
