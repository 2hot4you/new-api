# Grok Remote HTTP Status Diagnostic Plan

## Scope

Implement one backward-compatible diagnostic field. No provider request,
storage, security, billing, or client response behavior changes are permitted.

## Steps

1. Trace the current typed persistence error from the COS non-2xx branch to the
   adaptor log and identify the narrowest compatible extension point.
2. Add failing table tests for 401, 403, 404, 410, 429, and 502 plus a status-zero
   ordinary-error test and a safe adaptor log assertion.
3. Add a private status field, a status-aware diagnostic constructor/wrapper,
   and `GrokImagePersistenceRemoteStatus(err error) int` without changing the
   existing details extractor signature.
4. Attach `response.StatusCode` only in the remote image non-2xx branch and log
   the extracted integer on persistence failures.
5. Run focused tests, required package/full tests, race checks where relevant,
   vet, gofmt, and diff/secret review.
6. Run parallel antigravity and Claude review, resolve Critical/Warning findings,
   archive the CCG task, commit, push `develop`, monitor Development deployment,
   and verify only the public status endpoint/version.
