# Independent infrastructure review

Assessment: **Approved** after follow-up. No open Critical, Important or Minor findings. Read-only review of root infrastructure, not the reviewer's earlier model-test implementation. No Go test suite, SQLite/MySQL fixture, broad make test, external executor, or subagent was executed during this review. Writes are review reports only.

## Findings

### Critical

None.

### Important

None.

### Minor

None open. **M1 resolved:** the gate subprocess environment now explicitly sets `GOWORK="off"` (`deploy/tests/test_catalog_postgres_gate.py:633`), matching complete-suite and workflow settings. Originally the gate reconstructed os.environ without the override, allowing a local exported Go workspace to differ from CI. The common environment fix covers both ordinary commands and negative guard probes.

## Coverage and fail-closed audit

- Retrieved the authoritative failed output read-only with `gh run view 38047800232 -R 2hot4you/new-api --job 114200780655 --log-failed`. Extracted top-level `--- FAIL` names and compared them programmatically to both `FAILED_REGRESSIONS` and `catalog.GROUPS`. Result: **104 raw names, 104 declared names, zero missing declarations, zero extra declarations, zero missing mandatory selections**. This was an independent comparison with the failed run, not merely a comparison between two code lists.
- `validate_regression_inventory` (`deploy/tests/test_catalog_postgres_gate.py:583`) is called before any acceptance test commands (`deploy/tests/test_postgres_ci.py:211`). The exact gate has 31 groups. It requires every named parent to pass, requires a PostgreSQL child for matrix groups, rejects all skip/fail events and empty selections, and verifies command exit status (`test_catalog_postgres_gate.py:589–645`). Matrix command construction uses exact parent names plus `/^postgres$`; controller groups retain the existing catalogsynctest build tag.
- Full-suite execution uses 41 explicit root package paths, no root wildcard and no `-run` filter (`test_postgres_ci.py:20–62`, `:180–185`). Relaykit alone intentionally uses its full-module `./...`. Go-list metadata distinguishes genuine no-test packages from packages whose tests must pass; test-level skips are always rejected and an empty required test set cannot pass (`:188–202`, `:218–237`). DSN and Redis prerequisites are checked before invoking Go acceptance commands (`:208–210`).
- Inspected current full-package tests for SQL initialization, GORM/SQL opens, database migration, implicit database TestMain and model.DB assignments. The discovered implicit SQLite TestMain in `relay/channel/moliigrok` and SQL fixture in `relay/channel/task/jsplugin` are excluded from FULL_PACKAGES. Reviewed relaykit's only TestMain: it installs an in-memory media resolver and does not initialize a database. Followed the indirect controller import in setting/billing_setting's builtin tests: it reads in-memory option state rather than bootstrapping SQL. Redis dependencies are explicit: the common integration test requires TEST_REDIS_DSN, while several allowlisted adapter/wsmanager tests use isolated miniredis.
- The latest concurrent root update adds the three helper negative TestMain probes beside the model probes (`test_catalog_postgres_gate.py:613–625`). They keep PostgreSQL-only mode nonempty for missing DSN, malformed mode, and redirect/override DSN cases, require nonzero exit plus the expected guard diagnostic, and never intentionally enter the legacy TestMain branch.

## Workflow and scope audit

- PR and deploy workflows both call `make test-postgres` followed by `make test-postgres-race` (`.github/workflows/ci.yml:99–103`, `.github/workflows/deploy.yml:185–189`); both targets invoke the same runner (`makefile:84–89`). The complete-suite phases and mandatory selected PostgreSQL phases both receive race instrumentation in race mode.
- Legacy `make test` remains unchanged (`makefile:75–82`) but is no longer called by these acceptance workflows. The broad lifecycle race invocation was removed rather than relabeled as equivalent coverage. Existing frontend, vet, build, PostgreSQL/Redis services, source-SHA verification and release dependency boundaries remain unchanged in the reviewed diffs.
- Current wording accurately limits legacy coverage: the runner docstring and `LEGACY_DATABASE_PACKAGES` comment state these packages are not complete-suite runs and some have no mandatory selections. Runtime output says coverage is limited to the mandatory PG inventory (`test_postgres_ci.py:1–6`, `:171–177`, `:215`). This is not a claim that the historical broad root suite or every old fixture was migrated/passed.
- Follow-up reviewed `deploy/tests/deploy_test.sh:585–592`: all four normal/race workflow assertions now use newline-terminated exact command needles, so the race step cannot satisfy the normal-step assertion. Both workflows separately reject the old broad make-test and lifecycle race commands. This resolves the minor substring-match gap recorded in relay-review.md. No suites were repeated here, and this review does not assert a final full normal/race acceptance result.

Reviewed files: `.github/workflows/ci.yml`, `.github/workflows/deploy.yml`, `makefile`, `deploy/tests/test_catalog_postgres_gate.py`, and the untracked `deploy/tests/test_postgres_ci.py`; supporting reads were limited to the plan, archived diagnosis, exact failed CI output and focused suite-safety checks. Re-read the gate and runner after root's concurrent helper-probe/boundary-wording edits before recording this verdict.
