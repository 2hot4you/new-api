# Molii Auth Phase 1 Review

## Scope review

- No files under `web/` were changed.
- No administrator account, username, password, secret, or persistent test
  credential was created.
- The Go application was not added to Docker; the development Compose file
  remains PostgreSQL and Redis only.
- Existing authentication URLs, database tables, token claims, cookie name,
  cookie path, and `SameSite=Strict` policy were preserved.
- No push, merge, or pull request was performed.

## Findings

### Critical

None after fixes.

The cross-review found that the initial dashboard CORS header allowlist omitted
`Cache-Control`, which the existing browser HTTP clients send. The header and
an end-to-end router preflight assertion were added before completion.

### Warning

- A pre-existing middleware test closes the shared model database pool. One
  delegated review observed an order-dependent package failure, but repeated
  targeted and full repository test runs in the final workspace passed. This
  test-isolation cleanup is outside the authentication feature scope.
- `SESSION_SECRET` still has the upstream per-process random fallback when it
  is absent. Production documentation now makes a stable shared secret
  mandatory, but startup was not changed to fail closed to avoid an unrelated
  compatibility change.
- Cross-site frontend hosting remains intentionally unsupported because the
  refresh cookie remains `SameSite=Strict`. The supported topologies are a
  same-origin reverse proxy or same-site sibling origins with exact CORS and
  CSRF Origin configuration.

## Verification

- `gofmt` on all changed Go files: passed.
- `git diff --check`: passed.
- `go test ./controller ./middleware ./router`: passed.
- `go test ./...`: passed.
- `go vet ./...`: passed.
- `docker compose -f docker-compose.dev.yml config --quiet`: passed.
- PostgreSQL and Redis development containers: both healthy.
- Live HTTP validation on temporary port 3001:
  - configured-Origin `GET /api/status`: 200 with exact
    `Access-Control-Allow-Origin` and credentials;
  - configured-Origin `OPTIONS /api/user/login`: 204 with Authorization,
    Cache-Control, Content-Type and X-Auth-Session allowed;
  - invalid password-login request: 200 with
    `code=AUTH_INVALID_REQUEST`;
  - unconfigured-Origin `/api/status`: 403 with no CORS permission.

## Result

The existing session/JWT architecture was retained. The only backend
capabilities added are exact-Origin credentialed CORS for `/api` and stable
machine-readable codes for the first-stage password-auth and middleware error
paths. The independent frontend contract is documented in
`docs/molii-auth-api.md`.
