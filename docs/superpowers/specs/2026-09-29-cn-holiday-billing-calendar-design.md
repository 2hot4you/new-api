# China Holiday Billing Calendar Design

**Date:** 2026-09-29  
**Status:** Approved for implementation

## Outcome

The billing expression language gains `is_holiday(country, timezone)`. The first supported calendar is `CN` in `Asia/Shanghai`, backed by a versioned official State Council holiday schedule. An independent, read-only administrator page shows each model's effective daily billing periods.

## Billing semantics

- `is_holiday("CN", "Asia/Shanghai")` is true only for an explicitly listed statutory rest date.
- Ordinary weekends and makeup workdays return false. Whether they are peak or off-peak remains controlled by the expression's `weekday()` condition.
- Unsupported countries and uncovered years return false. This intentionally selects the higher weekday price when the calendar is unknown.
- Every expression evaluation captures one instant so all time and holiday functions see the same date and time.
- Holiday checks participate in request-rule tracing and fixed-price dependency detection.

## Calendar data

A single JSON document under `pkg/billingexpr/calendars/` is embedded by Go and imported by Web. It includes schema/version metadata, official source metadata, an explicit supported date range, statutory rest dates, and makeup workdays. Normal weekdays/weekends are derived only inside that range and only for display; they are never reinterpreted as statutory holidays.

The initial calendar starts on 2026-10-01 and contains the remaining official 2026 schedule from 国办发明电〔2025〕7号. Dates before 2026-10-01 are explicitly uncovered and `is_holiday` returns false. Updating a future period is a reviewed code/data change so billing history remains auditable.

## Admin page

`/billing-calendar` is available to administrators and above. It uses a dedicated read-only AdminAuth endpoint so it is not affected by the public Pricing navigation switch or user group filtering. The page provides:

- model selection for expression-priced models;
- month calendar with text and color markers for workday, weekend, statutory rest day, and makeup workday;
- a selected-day detail showing the calendar classification separately from the actual expression result;
- exact merged price intervals at one-minute precision, the finest precision supported by the expression language;
- an explicit warning for unsupported calendar years or expressions that cannot be safely visualized.
- an Apple Calendar `.ics` export for the selected model and visible month, containing each safely evaluated billing period with its tier, prices, and official day classification.

Pricing remains editable only in the existing Model Pricing interface.

## Safety

The frontend and backend use the same calendar file and share fixtures for parity. The UI does not infer prices from calendar labels: it evaluates the saved billing expression at every minute and merges identical adjacent results. Arbitrary expressions that cannot be represented safely degrade to an explanatory state rather than showing guessed prices.
