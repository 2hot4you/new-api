# Review

## Result

- 当前分页内的模型按厂商稳定分组。
- 厂商顺序取首次出现位置，同厂商模型保留原排序。
- 每个厂商使用独立响应式网格，新厂商从新行开始。
- 不显示厂商标题，不插入空白卡片。
- 缺少厂商信息的模型分别独立分组。
- 筛选、排序、分页、卡片内容和点击行为未改动。

## Verification

- 厂商分组测试先因模块缺失而失败，再转为通过。
- 网格集成测试先因厂商分组节点为 0 而失败，再转为通过。
- `bun test`: 267 passed, 0 failed.
- `bun run typecheck`: passed.
- Scoped `oxfmt --check`: passed.
- Scoped `oxlint`: passed.
- `git diff --check`: passed.
- 本地 `/pricing` 页面显示 7 个厂商分组、0 个厂商标题；每组纵向起点互不重叠。

按用户要求未调用 antigravity 或 Claude，未执行生产构建。
