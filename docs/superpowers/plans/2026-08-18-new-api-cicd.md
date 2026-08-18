# new-api CI/CD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and deploy immutable new-api images from `main` and `develop` with health-gated rollback and Telegram delivery notifications.

**Architecture:** GitHub Actions verifies the repository, builds a single amd64 GHCR image, and invokes a repository-owned server deployment script over SSH. Runtime application secrets remain in per-environment server files while Compose and deployment logic stay version controlled.

**Tech Stack:** GitHub Actions, Docker Buildx, GHCR, Docker Compose v2, Bash, SSH, Telegram Bot API

**Spec:** `docs/superpowers/specs/2026-08-18-new-api-cicd-design.md`

## Global Constraints

- `main` deploys only to production at `https://molii.co` and host port `3000`.
- `develop` deploys only to development at `https://dev.molii.co` and host port `3010`.
- The server architecture is `linux/amd64`.
- PostgreSQL, Redis, session, and crypto secrets never enter GitHub Actions or tracked files.
- Application ports bind only to `127.0.0.1`.
- GitHub Actions must not depend on the 1Panel API.

---

### Task 1: Deployment script contract and tests

**Files:**
- Create: `deploy/tests/deploy_test.sh`
- Create: `deploy/deploy.sh`

**Interfaces:**
- Consumes: `deploy.sh <production|development> <image-reference> <health-url>` and server `.env.runtime`.
- Produces: `.deploy.env`, a healthy Compose service, rollback to the previous image on failure, and a nonzero exit after rollback.

- [ ] Write a Bash integration test that stubs `docker`, `curl`, `flock`, and `sleep`; assert unsupported environments fail, missing/empty required runtime variables fail, healthy deploys keep the requested image, and unhealthy deploys restore the previous image.
- [ ] Run `bash deploy/tests/deploy_test.sh` and verify it fails because `deploy/deploy.sh` does not exist.
- [ ] Implement `deploy/deploy.sh` with strict mode, environment mapping, runtime-file validation, a server lock, immutable image deployment, bounded health waits, public status validation, log capture, and rollback.
- [ ] Run `bash deploy/tests/deploy_test.sh` and `bash -n deploy/deploy.sh` and verify both pass.

### Task 2: Runtime Compose contract

**Files:**
- Create: `deploy/docker-compose.cicd.yml`
- Create: `deploy/env/production.env.example`
- Create: `deploy/env/development.env.example`

**Interfaces:**
- Consumes: `.deploy.env` keys `IMAGE`, `HOST_PORT`, `CONTAINER_NAME`, `DEPLOY_ENV` and `.env.runtime` application settings.
- Produces: one isolated new-api container per environment, local-only port binding, health status, persistent data/logs, and bounded Docker logs.

- [ ] Add a Compose contract test to `deploy/tests/deploy_test.sh` that renders both environments and asserts `127.0.0.1:3000`/`127.0.0.1:3010`, distinct container names, and no PostgreSQL or Redis services.
- [ ] Run the test and verify it fails because the Compose file is missing.
- [ ] Add the Compose file and two non-secret runtime environment examples with environment-specific origins and node names.
- [ ] Run the deployment test and `docker compose ... config --quiet` for both environments.

### Task 3: GitHub Actions delivery pipeline

**Files:**
- Create: `.github/workflows/deploy.yml`

**Interfaces:**
- Consumes: push events on `main`/`develop`, GHCR's `GITHUB_TOKEN`, five SSH/Telegram repository secrets, and repository deployment assets.
- Produces: verified amd64 image digest, serialized environment deployment, and Telegram outcome notification.

- [ ] Add workflow contract assertions to `deploy/tests/deploy_test.sh` for branch triggers, GHCR permissions, digest deployment, fixed SSH known-host input, environment concurrency, and an `always()` notification job.
- [ ] Run the test and verify it fails because the workflow is missing.
- [ ] Implement verify, build, deploy, and notify jobs using the pinned action versions already present in the repository.
- [ ] Run the contract test and actionlint; correct all workflow syntax errors.

### Task 4: Operator runbook

**Files:**
- Create: `docs/deployment/molii-cicd.md`

**Interfaces:**
- Consumes: Ubuntu 24.04 with 1Panel-managed Docker/OpenResty, two prepared cloud databases, two Redis instances, DNS, and GitHub admin access.
- Produces: exact one-time server setup, runtime environment creation, GitHub secret setup, 1Panel proxy setup, first development release, production promotion, and rollback diagnostics.

- [ ] Document one-time user/directory preparation, secure key generation, `.env.runtime` installation, GitHub secret values, 1Panel proxy targets, and first-deploy order without including real secrets.
- [ ] Document GHCR package visibility, SSH host-key capture, Telegram bot setup, GitHub Environment protection, Uptime Kuma monitoring, and recovery commands.
- [ ] Run a placeholder/secret scan and manually check every referenced file, command, port, branch, URL, and secret name against the implementation.

### Task 5: Full verification and review

**Files:**
- Modify: `.ccg/tasks/design-new-api-cicd/task.json`
- Create: `.ccg/tasks/design-new-api-cicd/review.md`

**Interfaces:**
- Consumes: all files from Tasks 1-4.
- Produces: verified, reviewable feature branch ready for integration into `develop`.

- [ ] Run deployment tests, shell syntax checks, Compose rendering, actionlint, backend tests, independent relaykit tests, frontend typecheck, and production build; reproduce and record the pre-existing full frontend test baseline separately.
- [ ] Inspect `git diff --check`, tracked secret patterns, and the complete diff for scope and security.
- [ ] Record Critical/Warning/Info review results and the unavailable external-model limitation in `review.md`.
- [ ] Commit only the intended CI/CD files on `feat/new-api-cicd`.
