# rc.47 development acceptance

Accepted2026-10-26T10:59UTC. Actual development branch is **develop**; remote dev does not exist. No main merge/push or production deployment occurred.

## Source and successful delivery

- Official v1.0.0-rc.47: e835b1bfc43b565d625537830dbfb19328925134; full merge retains Molii customizations from develop e32fdcda17c5c48d26a461f6e4fd6c2ce862aa11.
- Accepted application source: **a85a9daa5ec112c0295cff48fd78bdf8af9d9269**, tree84f34b528e543f2abf815f87939d2b56582d7392. Development runtime **development-a85a9daa5ec1** verified publicly after repeated startup.
- [Application CI/CD40304552232](https://github.com/2hot4you/new-api/actions/runs/40304552232): every job succeeded. Sole release target development. Real PostgreSQL backup restored and verified offline, immutable image deployed successfully, repeated startup healthy. Deployment artifact result success, exact source SHA, rollback not_attempted.
- [Documentation CI/CD40289955728](https://github.com/2hot4you/new-api/actions/runs/40289955728): every job succeeded, development only. It ran on59bcf6b88; final repair changed no docs. Both commits have docs-site tree76c62cd9443f94ff70c67c6c2de8222242eaffb5 and docs workflow blob495814f12183ef42b65714349a1e2ea44cfdd2ec.

## Verification

- PostgreSQL only. Full mandatory database gate **normal1910 / race1910tests and subtests,48exact selections per mode**, zero skips, both local and final remote. Actual legacy seeded-before-bootstrap startup/repeat/pending/reload tests are mandatory.
- Independent root, relaykit and tokenkit builds/vet and complete audited ordinary/race package inventories passed. No broad unsafe database test execution or SQLite/MySQL testing.
- Full frontend **328files /3172tests** locally, final remote frontend/i18n gate success; types, scoped lint/format and three eligible brand builds passed. All13actual Go/frontend estimator parity vectors passed.
-33deployment Python contracts,197mock application assertions,40mock documentation assertions;141documentation nonbrowser and18local browser tests; actionlint on changed workflows passed.
- Independent native billing/catalog and deployment/security reviewers closed all confirmed findings. No external antigravity/Claude executor used.

## Public development acceptance

- `/api/status`: success, exact accepted source version. CNY display, recharge price7, USD exchange rate7 and quota_per_unit500000 preserved.
- `/api/pricing`: success, **46models /10vendors**. Model quote expressions, currencies, metadata, group membership, vendors and group configuration compare equal to pre-deployment projection. Exclude pricing_version and normalize only nondeterministic enable_groups order; no price differences suppressed.
- `/pricing`, `/pricing/gpt-image-2`, `/pricing/deepseek-flash` and `/docs/quick-start/`: HTTP200. Five referenced same-origin JavaScript assets: HTTP200 and JavaScript content types. Molii documentation content accepted.
- Zero charged provider calls and no administrator price/metadata/catalog/credential edits. Public projection comparison is not a claim of private database-byte comparison or live paid-provider settlement testing.

## Corrected release incident

Initial source59bcf6b88 passed code checks but [application40289955776](https://github.com/2hot4you/new-api/actions/runs/40289955776) failed startup and automatically rolled back successfully. Read-only development diagnostics identified legacy Grok/xai fixed quotes rejected during catalog reconstruction. Forward repair a85a9daa5 preserves proven unchanged stored quotes for reconstruction only; direct writes and selected task request guards remain strict before funding. Actual first/repeat startup and public configuration now pass. The failed source is not reported as deployed.

## Production boundary and closeout

- Protected main remains **ce7c4eb362728f3d55c72e80255f36829483b0a9**. Future main promotion requires separate explicit user approval.
- Upgrade code defaults main/bulk application AND documentation production releases to **molii.co, claudeye.com, model.claudeye.com only**. This policy takes effect on main when the user approves merging the upgrade; untouched current main is not claimed to contain it already.
- **aigc.ixiaozu.cn was never contacted, pulled, restarted or modified.** Future maintenance is standalone, separately approved, dated for the same Beijing day and limited to20:00–23:59. Runner/server time boundaries and denial-before-contact/write are tested.
- One task-only archive commit follows acceptance. Its paths are excluded from application deployment and do not match documentation triggers; deployed application source remains a85a9daa5. All reused worktrees, unrelated tasks and user untracked backups are preserved. The two exact task-owned local PostgreSQL/Redis fixtures were removed after acceptance; user development services were not touched.
