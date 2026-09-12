# Catalog Pricing Environment Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a safe, repeatable PostgreSQL catalog synchronization CLI for copying vendors, model marketplace metadata, and model pricing from development to independent production environments.

**Architecture:** A standalone `cmd/catalog-sync` binary exports a versioned JSON snapshot, computes a deterministic target-specific plan, and applies only a previously confirmed plan inside a transaction. Reusable logic lives in `internal/catalogsync`; stable vendor names and model names replace environment-local numeric IDs, and target-only models/prices remain untouched.

**Tech Stack:** Go 1.25, GORM v2, PostgreSQL driver, existing `model` structs and `common` JSON helpers, testify.

**Spec:** `.ccg/tasks/sync-catalog-pricing-environments/requirements.md`

## Global Constraints

- PostgreSQL only for this operational tool; the running application and its migrations remain unchanged.
- Dry-run is the default effect: only the explicit `apply` command may write.
- Never log DSNs, credentials, option values outside the catalog allowlist, or business records outside the catalog scope.
- Never delete vendor/model rows and never synchronize groups, channels, users, balances, logs, tasks, assets, Redis, or branding.
- Use existing persisted model/vendor structs and `common.Marshal`/`common.Unmarshal` helpers.

---

### Task 1: Versioned Snapshot and Scope Filtering

**Files:**
- Create: `internal/catalogsync/snapshot.go`
- Create: `internal/catalogsync/catalogsync_test.go`

**Interfaces:**
- Produces: `Snapshot`, `VendorRecord`, `ModelRecord`, `Export(ctx, db)`, `ReadSnapshot(io.Reader)`, `WriteSnapshot(io.Writer, Snapshot)`.

- [ ] Write failing tests proving export includes complete persisted vendor/model metadata, remaps model vendors by name, includes only the pricing option allowlist, and excludes group/site/security options.
- [ ] Run `go test ./internal/catalogsync -run 'TestExport' -count=1` and verify failure because the package is missing.
- [ ] Implement versioned snapshot types, allowlisted option selection, model/vendor loading, validation, and SHA-256 snapshot identity.
- [ ] Run the focused tests and verify they pass.

### Task 2: Deterministic Preview Plan

**Files:**
- Modify: `internal/catalogsync/snapshot.go`
- Modify: `internal/catalogsync/catalogsync_test.go`
- Create: `internal/catalogsync/plan.go`

**Interfaces:**
- Consumes: `Snapshot` from Task 1.
- Produces: `Plan`, `BuildPlan(ctx, db, Snapshot)`, `Plan.ConfirmationDigest()`.

- [ ] Write failing tests for vendor/model create/update/unchanged classification, vendor ID remapping, target-only preservation, scoped pricing-map reconciliation, and deterministic confirmation digest.
- [ ] Run `go test ./internal/catalogsync -run 'TestBuildPlan' -count=1` and verify the expected failure.
- [ ] Implement comparison and deterministic plan normalization without database writes.
- [ ] Run the focused tests and verify they pass.

### Task 3: Transactional Apply and Backup

**Files:**
- Create: `internal/catalogsync/apply.go`
- Modify: `internal/catalogsync/catalogsync_test.go`

**Interfaces:**
- Consumes: `Plan`, confirmed digest and backup writer.
- Produces: `Apply(ctx, db, Snapshot, expectedDigest, backupWriter)` and `ApplyResult`.

- [ ] Write failing tests proving mismatched confirmation is rejected, a backup is written before mutation, apply is transactional, target IDs are preserved, vendor references are remapped, target-only entries survive, and repeated apply is idempotent.
- [ ] Run `go test ./internal/catalogsync -run 'TestApply' -count=1` and verify the expected failure.
- [ ] Implement transactional upserts and option-map reconciliation using GORM, without row deletion.
- [ ] Run the focused tests and verify they pass.

### Task 4: Safe Command-Line Interface

**Files:**
- Create: `cmd/catalog-sync/main.go`
- Create: `cmd/catalog-sync/main_test.go`
- Modify: `Dockerfile`

**Interfaces:**
- Consumes: DSN only through a named environment variable, snapshot path, backup directory and confirmation digest.
- Produces: `export`, `plan`, and `apply` commands with JSON output and non-zero exit codes on unsafe/invalid use.

- [ ] Write failing command tests for help, missing DSN environment variable, default non-writing plan behavior, required confirmation, and credential-safe errors.
- [ ] Run `go test ./cmd/catalog-sync -count=1` and verify the expected failure.
- [ ] Implement argument parsing, PostgreSQL connection, atomic snapshot/backup file writing, command output, and include the utility binary in the application image.
- [ ] Run the focused tests and verify they pass.

### Task 5: Verification and Operator Guidance

**Files:**
- Create: `tools/catalog-sync/README.md`
- Modify: `.ccg/tasks/sync-catalog-pricing-environments/task.json`

**Interfaces:**
- Documents exact export, preview and apply commands using environment variables and separate execution on each server.

- [ ] Document development export, secure snapshot transfer, production preview, backup, apply, cache/application restart, and `/api/pricing` verification without real credentials.
- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./internal/catalogsync ./cmd/catalog-sync -count=1`.
- [ ] Run `go test ./...` and `go build ./cmd/catalog-sync`.
- [ ] Inspect `git diff` for scope and secret leakage.
- [ ] Record review results and archive the CCG task after verification.
