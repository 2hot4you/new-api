# 诊断结果

## 复现证据

- 用户当前操作页面为 `/system-settings/billing/model-pricing`。
- `deepseek-flash` 编辑器的“定价货币”选择为“站点货币（CNY）”。
- 线上 `/api/pricing` 仍返回 `deepseek-flash.billing_currency = "USD"`。
- 线上 `/api/status` 返回 `quota_display_type = "CNY"`、`usd_exchange_rate = 1`、`price = 7.3`。

## 根因

1. `PricingCurrencySelector` 只更新浏览器本地的 `model-pricing-preferences`，不调用服务端接口，也不更新模型元数据的 `billing_currency`。
2. `/pricing` 页面明确以模型元数据的 `billing_currency` 决定模型卡片显示 USD 或 CNY，所以编辑器选择不会影响模型广场。
3. 编辑器使用 `usd_exchange_rate` 做视觉换算，而开发环境该值为 1，因此 CNY/USD 切换时数值保持不变。
4. `price = 7.3` 是充值价格参数，并不是编辑器采用的 USD→CNY 展示汇率。

## 正确操作

若只需修改模型广场显示币种，应在 `/models/metadata` 编辑 `deepseek-flash` 的“计费币种”，并点击“保存元数据”。模型定价编辑器中的下拉框不是模型币种设置。

## 代码变更

无业务代码或线上配置变更。
