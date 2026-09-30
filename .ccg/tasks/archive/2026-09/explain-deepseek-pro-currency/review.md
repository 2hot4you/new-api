# 结论

- `deepseek-v4-pro-202606` 的线上模型元数据为 `billing_currency: "CNY"`，所以 `/pricing` 前端给表达式价格加人民币符号。
- `deepseek-flash` 的线上模型元数据为 `billing_currency: "USD"`，所以前端显示美元。
- 两者的表达式系数仍被后端计费引擎按 USD/百万 Token 解释；Pro “显示正常”只说明展示元数据符合预期，不证明实际扣费币种或金额正确。
- 无业务代码或线上配置变更。
