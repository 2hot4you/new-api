# 按次模型价格审计结论

## 结论

- `/system-settings/billing/model-pricing` 的“按次”价格不是全局必填项。
- 某模型选择按次模式并保存价格后，后端会把它作为该模型每次请求的基础价格，优先于 Token 倍率计费。
- 未配置按次价格时，异步任务依次尝试内置固定价格和模型倍率；全部缺失且未允许未定价模型时，请求会被拒绝。
- Seedance 的实际价格由 Molii AIGC 专用价格页控制，通用模型定价应保留默认倍率，不应随意切换为按次。
- Grok Imagine 的通用按次价格是内部 ¥1 计费锚点，实际图片、视频和工具价格由 Molii AIGC 专用价格页控制。

## 代码依据

- `relay/helper/price.go`：固定价格、默认价格、倍率回退与未配置错误。
- `relay/relay_task.go`：异步任务预扣费应用基础价格和适配器附加倍率。
- `relay/channel/task/starai/adaptor.go`：Seedance 专用价格换算。
- `relay/channel/moliigrok/adaptor.go`、`relay/channel/task/moliigrok/adaptor.go`：Grok 直接成本与 ¥1 锚点换算。
- `setting/ratio_setting/model_ratio.go`：Seedance 默认倍率和 Grok 默认固定价格锚点。

## 变更

- 本任务仅审计和说明，没有修改产品代码或配置。
