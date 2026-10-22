# Local acceptance — 2026-10-22

Pinned upstream7d8b91268dab69d7150f345c448b5e250163146a, business basec2442919e29403123cf8e5bcd15eea4db4dcc3e6. Final staged non-.ccg binary diff SHA256: d917345e823e356d4a34daa726ed16f235b0df583b58c2b11b0df9106c690d00. No unresolved index entries or diff whitespace errors. User untracked duplicate files and old task record are excluded.

## Passed

- Complete audited database-independent suites in normal and race: root43packages/2685tests/subtests, relaykit9/568, tokenkit1/7; zero test skips. These are not broad root cross-engine tests.
- Original mandatory PostgreSQL inventory in both modes:40exact selections/1621tests/subtests, zero skips. Final currency regression was added after those runs began, so the following final affected-group evidence is required alongside them.
- Final post-alias six affected PostgreSQL groups in both modes:117required top-level names/301tests/subtests, zero skips. Counts: model-store-pg144, model-store-direct10, model-runtime-pg81, model-runtime-neutral13, relay-catalog_pricing_capture_test34, relay-catalog_task_pricing_test19. Includes23currency provenance, provider and alias cases; no source changed during these final runs.
- Root Go1.26.10 vet/build; independent module vet/build; final Linux amd64 CGO-disabled greenteagc/moejs PGO executable build.
- Frontend326files/3053tests, typecheck, frozen install, i18n, scoped source lint/format and three brand-profile builds. Pinned Linux Bun1.4.0 Docker frontend builder target passed; full runtime image remains remote CI's responsibility.
- Deployment contracts:28Python tests/192mocked shell assertions. Backup helper11behavioral tests plus actual bounded offline main/log PostgreSQL restore. No live site deployment occurred in these tests.
- Native independent security/billing/ops reviews; all confirmed Important findings closed after observed regressions and scoped re-review. No antigravity/Claude external executor.

## Evidence paths

- Pure normal: /var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/postgres-ci-normal-xaxr3tw7
- Pure race: /var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/postgres-ci-race-3re93xbh
- Original PG normal: /var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/catalog-postgres-normal-zyeg2339
- Original PG race: /var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/catalog-postgres-race-r_6w78hi
- Final affected normal: /var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/rc46-builtin-currency-final-normal-944iakgw/summary.json
- Final affected race: /var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/rc46-builtin-currency-final-race-x1fpq_y2/summary.json

## Remaining gate

Push only the upgrade candidate, dispatch the existing trusted develop workflow with that immutable source SHA, backup_postgres=true and verify_repeated_startup=true. Remote CI must verify the final immutable source before deployment. Public status/pricing checks after deployment are non-mutating; real paid-provider billing and live catalog apply are not claimed. Production promotion remains separately authorized after dev acceptance. Legacy dashboard token retirement is30days after upgraded startup; existing model API keys are not that legacy credential.

## Final immutable-source remote acceptance

Run39762321755 verify job119769342926 succeeded29m26s. Sanitized completed job logs explicitly show SOURCE_SHA=cf9b53e2a101e70ff743ec238fca662b0fc856ed and:

- 326frontend files/3053tests passed at10:26:08UTC.
- Complete independent normal AND race: root43packages/2685tests/subtests, relaykit9/568, tokenkit1/7; zero skips.
- Final full mandatory normal1644tests/subtests/40exact selections passed10:34:41UTC, race1644/40 passed10:49:35UTC. Unlike the earlier local original inventory, these include the final23boundary cases on the same immutable commit.
- 28deploy Python tests/192mocked shell assertions passed10:49:39UTC.

Logs were read through the completed job endpoint and stripped of terminal control sequences; ordinary run-log CLI waits for the entire run including release. No log instructions were treated as commands. Release job119781547622 remains separate and pending deployment acceptance.
