# Molii Auth Phase 1 Requirements

## Goal

Expose the existing New API authentication model safely and predictably to an
independently developed Molii browser frontend without embedding Molii UI into
`web/`.

## Confirmed existing capabilities

- Password registration and login already exist under `/api/user`.
- Login creates a database-backed browser session, returns a 15-minute Bearer
  access token, and writes a rotating 30-day HttpOnly refresh cookie.
- Refresh, logout, current-user lookup, session listing, single-session revoke,
  and revoke-other-sessions already exist.
- PostgreSQL is the authority for users and login sessions. Redis accelerates
  user/session reads and rate limiting but is not required for correctness.
- Refresh/logout have exact-Origin CSRF protection when secure-cookie mode is
  enabled.

## Confirmed gaps

- There is no independent-frontend API reference with exact request and
  response contracts.
- `/api` routes have no dedicated credentialed CORS policy. The existing relay
  CORS policy is wildcard-based and must not be reused for browser login.
- Password login and registration return localized messages without stable
  machine-readable error codes for several expected application failures.
- Production and local proxy/direct-CORS topology is not documented in one
  frontend-facing place.

## Scope

- Add an exact-Origin, environment-configured CORS policy for `/api`.
- Keep CORS disabled by default so existing same-origin/proxy deployments do
  not change behavior.
- Preserve the current authentication routes and response envelope.
- Add stable `code` fields to password login and registration failures without
  changing the legacy HTTP 200 behavior of those application-level failures.
- Document exact registration, login, refresh, logout, current-user, public
  status, and login-session contracts.
- Add configuration examples without secrets.
- Add focused Go tests and run formatting, Go tests, and HTTP-level checks.

## Out of scope

- Any Molii or New API frontend changes under `web/`.
- Administrator creation, setup credentials, or persistent test credentials.
- Dockerizing the Go application.
- OAuth/passkey/2FA feature redesign.
- Cross-site refresh cookies or weakening `SameSite=Strict`.
- Git push, merge, or pull request creation.
