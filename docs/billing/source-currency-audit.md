# Model source-currency audit

This document records the models whose persisted pricing numbers are confirmed to be denominated in CNY. `models.billing_currency` is the source pricing currency; it is not the user-facing display currency.

At the currently configured rate of `1 USD = 7 CNY`:

- a CNY source amount of `¥x` is displayed as `¥x` when the site currency is CNY and settles as `x / 7` USD-equivalent quota before group ratio;
- a USD source amount of `$x` is displayed as `¥7x` when the site currency is CNY and settles as `x` USD-equivalent quota before group ratio;
- every request freezes the rate used for settlement and audit, so a later rate change does not reinterpret historical billing.

Legacy ratio and fixed-price models retain their existing USD semantics. The source-currency metadata described below applies to expression billing and the explicitly supported Seedance/Grok specialized pricing paths.

## Confirmed CNY models

| Models | Pricing mode | Source | Stored unit | Expected result with site CNY / rate 7 | Regression evidence |
| --- | --- | --- | --- | --- | --- |
| `minimax-m3` | Tiered billing expression | CNY | Expression coefficients in CNY per declared usage unit | A coefficient/result of `x` displays as `¥x` and settles as `x / 7` USD-equivalent | `model/pricing_currency_test.go`; expression settlement tests in `pkg/billingexpr`, `relay/helper`, and `service` |
| `qwen3.5-flash`, `qwen3.5-plus` | Tiered billing expression | CNY | Expression coefficients in CNY per declared usage unit | A coefficient/result of `x` displays as `¥x` and settles as `x / 7` USD-equivalent | dynamic CNY pricing tests under `web/src/features/pricing`; expression settlement tests in `pkg/billingexpr` and `service` |
| `deepseek-flash`, `deepseek-v4-flash-202605`, `deepseek-v4-pro-202606` | Tiered billing expression, including peak/off-peak branches where configured | CNY | Expression coefficients in CNY per million tokens (or the expression's explicitly declared usage unit) | Peak/off-peak expression result `x` displays as `¥x` and settles as `x / 7` USD-equivalent; the schedule changes the source result, not its currency | `model/model_billing_currency_migration_test.go`; dynamic CNY and model-card pricing tests under `web/src/features/pricing`; expression settlement tests in `pkg/billingexpr` and `service` |
| `doubao-seedance-2-0-260128`, `doubao-seedance-2-0-fast-260128`, `doubao-seedance-2-0-mini-260615`, `doubao-seedance-2-5-260628` | Administrator-maintained Seedance matrix compiled to task expressions | CNY | Matrix prices and generated expression output in CNY per million task tokens | Generated expression result `x` displays as `¥x` and settles as `x / 7` USD-equivalent | `setting/ratio_setting/starai_video_price_test.go`; `model/model_pricing_seedance_test.go`; `service/task_billing_test.go`; Seedance pricing tests under `web/src/features/pricing` |
| `grok-imagine-image`, `grok-imagine-image-quality`, `grok-imagine-image-2.0` | Grok direct image matrix | CNY | CNY per generated image plus configured input-image charge | Source subtotal `x` displays as `¥x` and settles as `x / 7` USD-equivalent; v2 audit records both values and the frozen rate | `service/grok_image_billing_test.go`; `relay/channel/moliigrok/adaptor_test.go`; Grok pricing and usage-log tests under `web/src/features` |
| `grok-imagine-video`, `grok-imagine-video-1.5` | Grok direct video matrix | CNY | CNY per generated video second, selected by operation/resolution | Source subtotal `x` displays as `¥x` and settles as `x / 7` USD-equivalent; async completion keeps the submission-time rate | `relay/channel/task/moliigrok/adaptor_test.go`; `service/task_billing_test.go`; Grok pricing and usage-log tests under `web/src/features` |

## Migration policy

- Migration marker `migration.model_billing_currency.v1` covers the previously confirmed MiniMax, Qwen, Seedance, and Grok rows.
- Migration marker `migration.model_billing_currency.v2` adds the three explicitly confirmed DeepSeek rows above.
- Each marker is written once. After it exists, an administrator's later source-currency edit is authoritative and is never overwritten by startup migration.
- Models not listed here are not changed to CNY automatically. Their exact metadata row remains authoritative; a missing row reads as legacy USD and cannot be used for a source-currency pricing write.

## Operational checklist

When adding another CNY-priced model:

1. Confirm the supplier invoice/price list currency and the expression or matrix unit.
2. Set the exact model metadata row to `CNY` in the same transaction as its pricing rule.
3. Add settlement, site-display, and audit-log regression coverage.
4. Add the exact identifier to this audit. Use a new one-time migration only if an already-deployed row needs backfilling; never infer the currency from the vendor or model prefix.
