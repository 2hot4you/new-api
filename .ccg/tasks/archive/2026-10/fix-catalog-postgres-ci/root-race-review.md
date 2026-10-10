# Independent root race-repair review

Assessment: **Approved**. No Critical, Important or Minor findings. Read-only source review; no tests repeated and no production code changes made by this reviewer.

Reviewed complete current diffs for exactly:

- relay/channel/api_request_test.go
- relay/channel/api_request_redirect_test.go
- relay/channel/api_request_getbody_test.go
- relay/channel/gemini/relay_gemini_usage_test.go
- relay/channel/gemini/relay_responses_test.go

The repair moves process-global Gin mode initialization to each package's single TestMain before m.Run (`relay/channel/api_request_test.go:16–20`, `relay/channel/gemini/relay_gemini_usage_test.go:21–25`). All per-test gin.SetMode writes in these package directories were removed. A focused source search of each package's direct Go files confirms exactly one TestMain and one SetMode call per package. The new entry points only set Gin mode and propagate m.Run's exit code; they do not initialize SQL or change the PostgreSQL-only suite boundary.

All existing t.Parallel calls, assertions, request/response setup and test bodies apart from mode writes are preserved. This addresses the reported concurrent global writes without serializing tests or changing production Gin behavior. It fits the incident-driven scope amendment in plan.md.

Controller evidence supplied for context: the full 41-package race run exposed Gin mode races only in these two packages; the focused complete-package rerun passed with race instrumentation in 2.904s and 2.066s, with no skips. These executions were not repeated by this reviewer. Final unified normal/race acceptance after code freeze remains controller-owned; this report makes no claim that the historical broad cross-engine suite passed.

Related follow-ups also source-verified: exact newline workflow normal/race contract needles and common GOWORK=off normalization are resolved in infra-review.md; helper/relay/controller strict disposable PostgreSQL DSN guards are resolved in relay-review.md.

## Subsequent MiniMax race follow-up

**Approved, no new findings.** Reviewed the complete additional diff for `relay/channel/minimax/adaptor_test.go`. It adds the package's single TestMain at lines 19–23, sets Gin test mode before m.Run, and removes the mode write from TestDoResponseForImageGeneration. All three t.Parallel calls and every business assertion remain intact. A focused search of the direct MiniMax package Go files confirms no other SetMode calls or TestMain declarations. This is the same narrowly scoped test-only ordering repair, with no SQL setup or production behavior change.

Independently read the second full-suite race log at `/var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/postgres-ci-race-_myzbgue/full-suite-0.jsonl`: the only package-level failure is MiniMax (line 4585); lines 4521–4542 show the Gin IsDebugging read versus SetMode write, pointing to the pre-fix adaptor test lines 55 and 90. TestConvertImageRequest and TestDoResponseForImageGeneration report failures after the detector output. Thus this follow-up has actual RED evidence, not merely speculative similarity to the earlier packages.

The controller's focused race count=3 verification was still running when this review was appended; no GREEN result is asserted here. No tests were repeated. Final combined normal/race verification remains controller-owned.
