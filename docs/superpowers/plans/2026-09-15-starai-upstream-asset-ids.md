# StarAI Upstream Asset IDs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Return and accept the upstream StarAI asset ID while preserving strict user-account ownership and legacy `asset-molii-*` compatibility.

**Architecture:** New bindings use a user-scoped Redis key derived from `user_id` and `upstream_id`; public APIs expose `upstream_id` as `id`. Legacy bindings remain readable through their existing Redis keys until TTL expiry. Every non-admin operation resolves through the authenticated user's scoped binding and never falls back to probing upstream IDs.

**Tech Stack:** Go, Gin, Redis, React/TypeScript, Docusaurus.

**Spec:** `.ccg/tasks/starai-upstream-asset-ids/requirements.md`

## Global Constraints

- Authorization boundary is the user account, not the token ID.
- Preserve channel ID and channel-key fingerprint checks.
- Preserve seven-day expiration and COS cleanup behavior.
- Never query an unbound raw upstream asset ID.
- Existing `asset-molii-*` references remain valid until their Redis TTL expires.

---

### Task 1: User-scoped asset binding storage

**Files:**
- Modify: `service/starai_asset.go`
- Test: `service/starai_asset_test.go`

**Interfaces:**
- Consumes: `StarAIAssetBinding`, authenticated `userID`, public asset ID.
- Produces: scoped save/get/delete/list functions that accept upstream IDs and legacy IDs.

- [x] Write failing tests proving two users can hold the same upstream ID without overwriting and cannot read each other's binding.
- [x] Run the service tests and verify the new tests fail for the expected Redis-key collision or lookup behavior.
- [x] Implement scoped Redis keys, retain legacy reads, and remove the generated ID for new bindings.
- [x] Run service tests and verify both new and legacy lifecycle tests pass.

### Task 2: Public API and request resolver contract

**Files:**
- Modify: `controller/starai_asset.go`
- Modify: `relay/channel/task/starai/adaptor.go`
- Test: `controller/starai_asset_test.go`
- Test: `relay/channel/task/starai/adaptor_test.go`

**Interfaces:**
- Consumes: upstream ID returned by StarAI and authenticated user identity.
- Produces: create/get/delete responses and `asset://<upstream-id>` resolution protected by local ownership.

- [x] Write failing controller and adaptor tests for upstream ID responses, cross-user rejection, same-user cross-token access, and legacy URI support.
- [x] Run focused tests and verify failure is caused by the old `asset-molii-*` contract.
- [x] Update controller responses and resolver validation without adding a direct-upstream fallback.
- [x] Run focused tests until green.

### Task 3: Listing, administration, frontend, and documentation

**Files:**
- Modify: `web/src/features/temporary-assets/**`
- Modify: `docs-site/docs/api-reference/assets.mdx`
- Modify: `docs-site/docs/guides/temporary-assets.mdx`
- Modify: `docs-site/docs/guides/seedance-multimodal.mdx`
- Modify: `docs-site/docs/api-reference/seedance.mdx`
- Modify: related documentation contract tests and customer delivery documents where the old prefix is asserted.

**Interfaces:**
- Consumes: public upstream asset IDs from the API.
- Produces: UI copy/copy actions and documentation examples using `asset://<upstream-id>`.

- [x] Update existing tests first so old hard-coded `asset-molii-*` examples fail.
- [x] Update UI assumptions and documentation examples while documenting user ownership.
- [x] Run frontend and Docusaurus contract tests.

### Task 4: Verification and compatibility audit

**Files:**
- Modify only files required by failing verification.

**Interfaces:**
- Consumes: all prior task outputs.
- Produces: verified implementation and review record.

- [x] Run `go test ./service ./controller ./relay/channel/task/starai`.
- [x] Run affected frontend tests and type checks.
- [x] Run Docusaurus content contract tests/build relevant to assets and Seedance.
- [x] Inspect `git diff` for secrets, unrequested changes, and loss of legacy compatibility.
- [x] Record review findings in `.ccg/tasks/starai-upstream-asset-ids/review.md`.
