# Molii Auth Phase 1 Plan

1. Record the exact existing route, controller, model, database, Redis, cookie,
   CSRF, and proxy behavior.
2. Add validated `DASHBOARD_CORS_ALLOWED_ORIGINS` configuration and install a
   dedicated credentialed CORS middleware on `/api` before route registration.
3. Add stable error codes to password login and registration responses while
   preserving the established JSON envelope and compatibility behavior.
4. Write an independent-frontend authentication API reference with exact
   payloads, headers, lifecycle guidance, proxy/CORS strategy, production
   examples, and error handling.
5. Add focused unit/integration tests, run `gofmt`, targeted tests, the full Go
   test suite, `go vet`, Compose validation, and local HTTP contract checks.
6. Review the final diff, record findings, update task state, and archive the
   completed CCG task without pushing.
