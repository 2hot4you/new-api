# Review: three-environment, dual-production branding and deployment

## Scope reviewed

- `develop` maps only to `development` and `dev.molii.co`.
- `main` maps independently to `production-molii` (`molii.co`) and
  `production-ixiaozu` (`aigc.ixiaozu.cn`).
- Application and documentation releases select separate GitHub Environments,
  SSH credentials, deployment directories, runtime environment files, health
  URLs, branding values, artifacts, result records, rollback state, and
  Telegram notifications.
- The application supports build-time title, description, logo, favicon,
  Apple touch icon, banner brand, and default font selection. Molii retains its
  established defaults. The iXiaozu profile rejects incomplete configuration.
- The colored homepage brand renderer accepts arbitrary Unicode graphemes and
  preserves the alternating pink/blue sequence and accessible full text.
- Docusaurus uses the selected origin, API base URL, title, tagline, navbar,
  footer, font, logo, favicon, social image, canonical URLs, and sitemap.
- Downloaded documentation brand assets are HTTPS-only in production, bounded
  in size, MIME checked, sanitized for SVG active content, and atomically
  replaced.

## Verification evidence

- `make test`: root and relaykit Go modules passed.
- `go vet ./...`: passed.
- `go build ./...`: passed.
- `cd relaykit && go vet ./... && go build ./...`: passed.
- `bash deploy/tests/deploy_test.sh`: 86/86 assertions passed.
- Changed web tests: 35/35 passed.
- `cd web && bun run typecheck`: passed.
- Molii and iXiaozu application production builds: passed; generated iXiaozu
  HTML contained the selected title and icons and no Molii title fallback.
- `cd docs-site && bun test`: 153/153 passed.
- `bun run api:lint`: public OpenAPI passed validation.
- `bun run catalog:check`: generated Provider/model documentation matched the
  catalog snapshot.
- `bash docs-site/deploy/deploy_test.sh`: 26/26 assertions passed.
- Molii and iXiaozu `/docs/` production builds: passed.
- Molii and iXiaozu `/docs/` internal link crawls: 43/43 links passed for each
  brand.
- Generated iXiaozu docs used the iXiaozu canonical/API origin, brand assets,
  title, navbar/footer identity, and sans font; no Molii deployment URL or
  Molii brand asset fallback was present in the inspected output.
- Both full application Docker images were built successfully during image
  validation. A later redundant Molii rebuild was cancelled after its remote
  dependency download stalled; this was not a compile or test failure.
- GitHub workflow files parsed as YAML. `actionlint` is not installed locally.
- `git diff --check origin/develop`: passed after normalizing plan-file endings.
- Diff credential scan found no embedded API key or private key. The only
  password-like workflow value is GitHub's ephemeral `${{ github.token }}`.

## Known baseline and environment limitations

- The combined command
  `go test -race ./common ./middleware ./service ./controller ./relay ./router -count=1`
  failed once because concurrently running packages shared legacy SQLite/global
  test state (duplicate token values and missing tables). Re-running
  `go test -race ./controller -count=1` passed. This change does not modify Go
  application or test code, and the normal complete Go suite passed. The
  production contract remains PostgreSQL plus Redis; SQLite is not part of the
  deployment acceptance criteria.
- `bun run check` with Docusaurus served at the local root reports the links
  back to the main application as local 404s. Both real deployment shapes use
  `/docs/`; their brand-specific builds and link crawls passed.
- External CCG model analysis/review was unavailable because
  `~/.claude/bin/codeagent-wrapper` is not installed. The user selected inline
  implementation, so no subagent review was dispatched. Review used local
  source inspection, contract tests, production builds, and generated-output
  inspection.
- No real server, PostgreSQL database, Redis instance, user account, channel,
  or production DNS state was changed by this branch.

## Operator work still required

- Create/configure the `development`, `production-molii`, and
  `production-ixiaozu` GitHub Environments using the exact variables and
  secrets in `docs/deployment/molii-cicd.md`.
- Put independent runtime `.env` files on each server, with separate
  PostgreSQL, Redis, session/encryption secrets, users, and channels.
- Provide public HTTPS logo, favicon, and social-image source URLs for each
  documentation target and site-local application brand asset URLs.
- Configure each reverse proxy and DNS entry, then run the documented staged
  acceptance flow before merging `develop` into `main`.
