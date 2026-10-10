# Whole-change integration review

Source review against `275423c64`, including committed model/controller/relay fixtures, current workflow/make/gate diffs, the new `deploy/tests/test_postgres_ci.py`, deployment contract changes, and the bounded Gin race repair. No whole suites repeated, no external executors, no production changes made by this reviewer. Complementary independent infrastructure/relay reviews are recorded separately; this reviewer did not treat self-review of Task 2 as independent approval.

## Verdict

No remaining Critical or Warning source findings. Ready for root's fresh complete normal/race acceptance, contract/vet/build checks and final delivery checks. This is not a claim that an in-progress acceptance run or post-push workflow has completed.

The review found and caused repair of standalone fixture isolation and controller global cleanup rather than relying only on the outer runner. Helper/relay/controller guard tests now reject database/host/port/service query overrides before connections; only a single `sslmode=disable` query value is accepted. The helper TestMain rejects every nonempty invalid mode before its legacy initialization. Actual bootstrap/recovery and desired/effective plugin parity remain; no production ready-state bypass appears.

## Coverage boundary and inventory

- All **104 mandatory incident names** from failed run 38047800232 remain declared and selected. The independent infrastructure reviewer compared these with the authoritative failed log, not just another code list.
- The repair additionally keeps **24 converted tests** beyond the original failures in mandatory acceptance. At that expansion the inventory was **31 groups / 367 top-level tests**. The three subsequently added standalone DSN guard tests raise the current source inventory to **31 groups / 370 distinct top-level tests**. Read-only metadata import confirmed those counts and 104 incident names; no tests were executed by that import.
- Both PR and deploy workflows use the same `make test-postgres` and `make test-postgres-race` entry points. Mandatory groups use exact names, require selected PostgreSQL matrix children, reject missing/empty/failed/skipped cases and check process status. Race mode reaches complete independent suites and every mandatory PostgreSQL group.
- The runner retains complete, unfiltered suites for **41 audited database-independent root packages** and the complete relaykit module. Genuine no-test packages are distinguished from missing test execution. It does not enter the known implicit SQLite TestMain in relay/channel/moliigrok, the task/jsplugin SQL fixture, or broader legacy database suites.
- **Reduced legacy-coverage boundary is explicit:** the historical broad `make test` and broad lifecycle race suites are not equivalent to this gate and are not claimed passing. Model/controller/relay/helper and other listed legacy database packages receive only their mandatory PostgreSQL selections; some excluded packages receive no runtime tests. Their old engine-specific suites remain unchanged and available outside deployment acceptance. This incident repair does not migrate approximately 98 legacy fixture files.

## Integration checks from source

The local and workflow runners normalize `GOWORK=off`; the earlier infrastructure consistency nit is resolved. Root DSN validation is sslmode-only before complete-suite or selected commands. Redis is an explicit complete-common-suite prerequisite. Root wildcard full-suite execution is prohibited by the allowlist helper and its unit tests. The retained catalogsynctest tag stays on controller groups.

Deployment contracts now use newline-terminated normal/race command needles, resolving the earlier substring false-positive nit, and separately reject the old broad commands. Unchanged frontend, vet/build, service and release source-SHA/dependency safeguards remain in workflow diffs.

The model concurrency change retains snapshot/vendor-count/barrier/revision assertions while bounding pre-callback errors, cancellation and worker cleanup; the PostgreSQL rejection case proves no committed row or revision. Details are in model-review.md.

Controller bootstrap, plugin parity, rollback and global cleanup were independently reviewed in controller-review.md. The only modified business-test check is equivalent or strengthened; no failure is removed from mandatory selection. Request-only malformed expressions and stale usage schemas remain at their original request boundary after actual startup, rather than being adopted as invalid startup configuration.

The Gin race repair changes only test process setup: `gin.SetMode(gin.TestMode)` runs once in each package TestMain before m.Run. All former test-body mode writes in relay/channel and relay/channel/gemini are removed, while t.Parallel, test names, handlers and assertions remain. The two TestMain bodies open no database. This addresses the root-observed full-race RED without serializing/removing tests.

## Final delivery conditions

Root must retain fresh full-run evidence for the final code state, resolve any actual failures, stage only intended paths, archive task records and confirm nonforce fast-forward push/release status. Unrelated untracked web files must remain untouched. No additional implementation expansion is recommended absent a concrete remaining failure.
