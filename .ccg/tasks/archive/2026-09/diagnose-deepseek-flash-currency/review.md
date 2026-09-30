# 诊断结果

## 线上证据

- `deepseek-flash` 的 `/api/pricing` 返回 `billing_currency: "USD"`。
- `deepseek-v4-pro-202606` 的 `/api/pricing` 返回 `billing_currency: "CNY"`。
- 两个模型均使用人民币数值的峰谷表达式，但表达式本身不携带币种。

## 数据链路

1. 定价接口从模型元数据的 `billing_currency` 字段读取币种。
2. 前端价格卡片直接使用该字段决定显示 `$` 或 `¥`，不会通过表达式数值推断币种。
3. 可视化价格编辑器只保存计费表达式；币种属于模型元数据，需要在“模型信息”中另行保存。

## 根因

`deepseek-flash` 当前持久化的模型元数据币种仍为 USD，而 `deepseek-v4-pro-202606` 已保存为 CNY。因此前端行为符合接口数据，不是价格卡片格式化错误。

## 处理建议

在管理员后台 `/models/metadata` 编辑 `deepseek-flash`，将“计费币种”选择为 `CNY (¥)`，点击“保存元数据”。保存会刷新定价缓存，计费表达式无需修改。

## 代码变更

无业务代码或线上配置变更。
