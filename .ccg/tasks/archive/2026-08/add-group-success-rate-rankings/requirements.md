# Group success-rate rankings requirements

## Outcome

- Show weighted real request success rates for configured groups on `/rankings`.
- Use the same real performance data in `/keys` instead of manually configured recommendation scores.
- Remove recommendation metadata completely, including the input on `/system-settings/billing/group-pricing`, API fields, backend types, validation, persistence, and stale persisted values.

## Metrics contract

- Calculate each group rate as `sum(success_count) / sum(request_count) * 100` across all models and buckets in the selected period.
- Never average already-computed model percentages.
- Merge persisted database buckets with current in-memory buckets without double counting.
- Periods match rankings: `today` = 24 hours, `week` = 7 days, `month` = 30 days, `year` = 365 days.
- A configured group with no request samples has no success-rate value and displays `No requests`; it does not receive a fallback rate and does not participate in numeric ranking.
- `/keys` uses today's data and displays either the real rate or `No requests`.

## UI contract

- `/rankings` shows every currently configured group, real success rate, request count, period-aware copy, and explicit no-request state.
- Groups with data are ordered by success rate descending, then request count descending; no-request groups follow in configured metadata order.
- `/keys` selected rows, group picker, and cross-group details replace recommendation stars with a compact real-success-rate badge.
- `/group-pricing` no longer renders, validates, or saves recommendation input.
- All new user-visible text is translated through i18n.

## Compatibility and safety

- Keep SQLite, MySQL, and PostgreSQL compatible queries.
- Do not change routing order, billing ratios, token selection semantics, or request retry behavior.
- Do not fabricate success data when metrics are unavailable.
- Do not expose raw request records or sensitive values.
- Normalize existing group metadata JSON so persisted `recommendation` properties are removed.
