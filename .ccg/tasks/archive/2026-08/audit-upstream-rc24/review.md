# New API v1.0.0-rc.24 upstream audit

## Baseline

- Current branch: `feat/molii-auth`
- Current upstream base: `v1.0.0-rc.23` (`0ab02020`)
- Target tag: `v1.0.0-rc.24` (`5c3abffe`)
- Divergence: Molii has 220 unique commits; rc.24 has 8 unique commits.
- rc.23 → rc.24: 35 files, 2,407 insertions, 552 deletions.
- Dry-run merge: one textual conflict in `relay/image_handler.go`.

## Recommended updates

### P0

1. HTTP/2 request-body replay and redirect handling (`d6b5ce99`, `ea4f0210`).
   - Current Molii code still uses `ReaderOnly`, the old `NewOutboundJSONBody` signature, and a same-reader `GetBody` closure in task requests.
   - Merge must preserve Molii Grok image sanitization, request-body logging policy, model mapping, and COS behavior in `relay/image_handler.go`.
2. User/access-token and affiliate accounting concurrency hardening (`0cd9dc85`).
   - Current Molii code still updates access tokens through a stale user snapshot and does not omit affiliate fields/access token from generic user updates.
3. Per-user critical rate limiting for access-token rotation and affiliate transfer (`1da23d6b`).
   - Molii already has the user-keyed limiter factory, but these two routes are not wired to it.
4. Redemption-code editable quota precision (`e926e5ca`).
   - Relevant to Molii's CNY/custom currency display and fractional redemption values.

### P1

1. Native Claude/Gemini channel-test request formats (`b941253a`).
2. Expanded fetched-model categorization (`c9bc0386`), including xAI, MiniMax, Qwen, Doubao/Seedance and many other vendors.

### Skip unless needed

- GitCode release synchronization workflow (`5c3abffe`): not needed for Molii runtime unless that mirror is used.

## Deployment impact

- No database migration.
- No new environment variables.
- No Go or Bun dependency update.
- No direct upstream change to Molii pricing catalog, async billing outbox, Grok adaptor, Seedance adaptor, or COS persistence.
- After the release merge, run focused auth/user accounting, relay HTTP/2 replay, channel test, redemption-code, Grok image, Seedance/Grok async task, full Go, and frontend tests.

## Unreleased upstream

Upstream `main` is 18 commits ahead of rc.24. Do not merge those unreleased commits as part of the rc.24 upgrade; evaluate them separately or wait for the next release.

No antigravity or Claude models were called, per user instruction. No business code was changed in this audit.
