# Seedance 任务定价提示诊断

## 结论

- dev 的 Seedance 模型已返回 `video_pricing` 专用视频价格矩阵，并非没有价格。
- 通用模型价格列表仅以 `billing_mode=tiered_expr` 且存在 `billing_expr` 判断任务定价是否配置，因此对专用矩阵模型产生误报。
- 不应仅为消除提示而强制写入任务表达式；ByteDance Seedance 原生转售适配器按实际 Token 与提交时倍率快照结算，并不提供通用任务表达式所要求的用量事实接口。
- dev 当前 Mini 与 2.5 的基础模型倍率分别为 23、70；按 `QuotaPerUnit=500000` 换算为 46、140 CNY/百万 Token，高于代码默认基线 23、70 CNY/百万 Token，需要单独校正配置。

## 建议

1. 通用列表将存在有效 `video_pricing.rows` 的模型视为“专用任务定价已配置”，并显示“专用视频定价”。
2. 编辑入口引导至 Seedance 专用定价页，不提示用户创建通用任务表达式。
3. 将 dev 的 `doubao-seedance-2-0-mini-260615` 与 `doubao-seedance-2-5-260628` 基础输入价核对并分别恢复为 23、70 CNY/百万 Token（对应模型倍率 11.5、35）。

## 验证

- dev `/api/pricing` 返回四个 Seedance 模型的 `billing_usage_schema` 与 `video_pricing`。
- Seedance 价格设置测试通过。
- ByteDance Seedance 结算测试通过。
- Seedance 模型卡价格展示测试通过。
