# Requirements

## Rankings group success rates

- Keep the existing real success-rate calculation and period selector.
- Sort groups with data by success rate descending, then request count descending.
- Place groups without data after measured groups and preserve the configured group-pricing order.
- Return and render each group's configured Lobe icon key and description.
- Do not restore recommendation/rating fields or synthetic fallback success rates.

## API key model restrictions

- In the API-key create/update drawer, render the configured `model.icon` before each model ID in the selectable list and selected chips.
- Resolve icons from pricing/model metadata, never from provider metadata.
- Keep the existing model restriction payload contract.

## API key IP/CIDR restrictions

- Replace the textarea-only experience with an input, explicit add button, removable chips, and visible validation feedback.
- Accept IPv4, IPv6, IPv4 CIDR, and IPv6 CIDR.
- Support pasting multiple values separated by commas, whitespace, or newlines.
- Ignore duplicates and reject invalid entries without silently submitting them.
- Preserve the existing backend payload and database contract by serializing accepted values as newline-delimited `allow_ips`.
- All new user-visible text must use existing i18n conventions.

## Delivery

- Work only in the develop worktree.
- Add behavior-focused tests before implementation and observe them fail.
- Run affected backend/frontend tests, typecheck, scoped lint, i18n checks, and production builds.
- Commit and push to `origin/develop` only after confirming no remote drift.
