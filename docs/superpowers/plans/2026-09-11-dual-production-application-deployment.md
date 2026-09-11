# Dual-Production Application Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy `develop` only to `dev.molii.co` and deploy every accepted `main` revision independently to both `molii.co` and `aigc.ixiaozu.cn`.

**Architecture:** A prepare job emits a JSON target matrix. Verification runs once, while brand-specific image build and SSH deployment run per target with `fail-fast: false`. GitHub Environments isolate public brand variables and infrastructure secrets; each target performs its own health gate, rollback, and Telegram result notification.

**Tech Stack:** GitHub Actions, Docker Buildx, GHCR, Docker Compose v2, Bash, SSH, Telegram Bot API

**Spec:** `docs/superpowers/specs/2026-09-11-three-environment-multi-site-branding-design.md`

## Global Constraints

- Start from `origin/develop` in an isolated worktree.
- Preserve development-only candidate source, PostgreSQL backup, and repeated-startup controls.
- `main` always maps to exactly `production-molii` and `production-ixiaozu`.
- A failure in one production matrix entry must not cancel or roll back the other.
- PostgreSQL, Redis, session, crypto, channels, and users remain target-local.
- Production is not deployed from `develop` or an arbitrary `source_ref`.

---

### Task 1: Generalize deployment targets and runtime examples

**Files:**
- Modify: `deploy/deploy.sh`
- Modify: `deploy/tests/deploy_test.sh`
- Rename: `deploy/env/production.env.example` to `deploy/env/production-molii.env.example`
- Create: `deploy/env/production-ixiaozu.env.example`
- Modify: `deploy/env/development.env.example`

**Interfaces:**
- Consumes: `deploy.sh <development|production-molii|production-ixiaozu> <image-reference> <health-url>`.
- Produces: target-specific directory, port, container, Compose project, public health validation, and local rollback.

- [ ] **Step 1: Extend test fixtures to all three targets**

Use these exact mappings in table-driven tests:

```text
development        /opt/molii/development  3010  molii-development         https://dev.molii.co/api/status
production-molii   /opt/molii/production   3000  molii-production          https://molii.co/api/status
production-ixiaozu /opt/ixiaozu/production 3000  ixiaozu-production        https://aigc.ixiaozu.cn/api/status
```

Assert a wrong target/health pair fails before Docker commands run.

- [ ] **Step 2: Run the deployment contract and confirm the third target is rejected**

Run: `bash deploy/tests/deploy_test.sh`

- [ ] **Step 3: Implement the three-target case statement**

Keep the current strict validation, `flock`, immutable digest, container health wait, public health check, log capture, and previous-image rollback. Use target-specific `DEPLOY_DIR`, `HOST_PORT`, `CONTAINER_NAME`, `EXPECTED_HEALTH_URL`, and `COMPOSE_PROJECT_NAME`.

- [ ] **Step 4: Add iXiaozu's non-secret runtime example**

The example contains empty `SQL_DSN`, `REDIS_CONN_STRING`, `SESSION_SECRET`, and `CRYPTO_SECRET`, plus:

```dotenv
TZ=Asia/Shanghai
ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
NODE_NAME=ixiaozu-production
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://aigc.ixiaozu.cn
```

- [ ] **Step 5: Run deployment and shell checks**

Run: `bash deploy/tests/deploy_test.sh && bash -n deploy/deploy.sh deploy/tests/deploy_test.sh`

- [ ] **Step 6: Commit**

```bash
git add deploy/deploy.sh deploy/tests/deploy_test.sh deploy/env
git commit -m "feat: add independent ixiaozu deployment target"
```

### Task 2: Convert the application workflow to a target matrix

**Files:**
- Modify: `.github/workflows/deploy.yml`
- Modify: `deploy/tests/deploy_test.sh`

**Interfaces:**
- Consumes: `github.ref_name`, optional development-only workflow inputs, GitHub Environment variables, and per-environment secrets.
- Produces: one development release or two independent production releases.

- [ ] **Step 1: Write workflow contract assertions**

Assert the workflow contains:

```yaml
strategy:
  fail-fast: false
  matrix:
    target: ${{ fromJSON(needs.prepare.outputs.targets) }}
environment:
  name: ${{ matrix.target.environment }}
```

Assert `develop` emits one `development` entry and `main` emits `production-molii` plus `production-ixiaozu`. Assert no shared production SSH secret name is used outside the selected GitHub Environment.

- [ ] **Step 2: Run the contract and confirm the single-target workflow fails it**

Run: `bash deploy/tests/deploy_test.sh`

- [ ] **Step 3: Emit a JSON matrix in `prepare`**

Each object contains `id`, `environment`, `image_tag`, and `brand_profile`. Domain, deploy directory, and public brand values come from that object's GitHub Environment `vars`, not repository-wide secrets.

- [ ] **Step 4: Keep verification single-run and make release matrix-driven**

The release job checks out the resolved source SHA, builds the target brand with `VITE_SITE_*` build arguments, pushes immutable target tags, captures the target digest, configures target-environment SSH, uploads deployment assets, and calls the generalized deployment script.

- [ ] **Step 5: Preserve development-only manual controls**

Reject `source_ref`, `backup_postgres`, or `verify_repeated_startup` unless `github.ref_name == 'develop'` and the sole matrix target is `development`.

- [ ] **Step 6: Add per-target notification and release summary**

Use a final `if: ${{ always() }}` step in each target release to report site ID, domain, SHA, build/deploy result, and Actions URL. Add a summary job with `if: always()` that exposes full success, failure, cancellation, or partial success without changing successful target state.

- [ ] **Step 7: Run workflow syntax and contract checks**

Run: `bash deploy/tests/deploy_test.sh`, `actionlint .github/workflows/deploy.yml` when installed, and parse the workflow with the repository's existing YAML fallback when actionlint is unavailable.

- [ ] **Step 8: Commit**

```bash
git add .github/workflows/deploy.yml deploy/tests/deploy_test.sh
git commit -m "ci: deploy main to both production sites"
```

### Task 3: Pass brand inputs into immutable images

**Files:**
- Modify: `Dockerfile`
- Modify: `.github/workflows/deploy.yml`
- Modify: `deploy/tests/deploy_test.sh`

**Interfaces:**
- Consumes: eight `VITE_SITE_*` public build arguments.
- Produces: one brand-correct embedded frontend image digest per matrix target.

- [ ] **Step 1: Add contract assertions for all public brand arguments**

Assert the workflow sends profile, title, description, logo, favicon, Apple icon, banner brand, and default font, and that the Dockerfile declares and forwards each only to the frontend builder stage.

- [ ] **Step 2: Run the contract and confirm the current Dockerfile omits brand arguments**

Run: `bash deploy/tests/deploy_test.sh`

- [ ] **Step 3: Add frontend-only Docker arguments**

Declare the eight arguments in the Bun builder, expose them only for `bun run build`, and do not copy them into the final container environment. Update the OCI title/description to neutral multi-site wording while retaining source, revision, version, license, provenance, and SBOM labels.

- [ ] **Step 4: Build both production variants locally with non-secret fixture values**

Run two `docker build` commands with different `VITE_SITE_PROFILE` values and verify each image serves only its own initial HTML metadata.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile .github/workflows/deploy.yml deploy/tests/deploy_test.sh
git commit -m "ci: build site-specific application images"
```

### Task 4: Update the operator runbook

**Files:**
- Modify: `docs/deployment/molii-cicd.md`

**Interfaces:**
- Consumes: three GitHub Environments, three server runtime files, DNS, TLS, reverse proxies, PostgreSQL, Redis, and public brand assets.
- Produces: repeatable development acceptance and dual-production promotion instructions.

- [ ] Document the exact promotion path: local → `develop` → `dev.molii.co` acceptance → PR/merge to `main` → two production releases.
- [ ] Document all GitHub Environment variables and secrets separately for `development`, `production-molii`, and `production-ixiaozu` without real values.
- [ ] Document `/opt/molii/development`, `/opt/molii/production`, and `/opt/ixiaozu/production`, file permissions, reverse-proxy targets, health URLs, and first-deploy order.
- [ ] Document partial-success handling: repair/re-run only the failed target; never copy databases or runtime configuration between sites.
- [ ] Document Uptime Kuma monitors for all three `/api/status` endpoints and Telegram recovery notifications.
- [ ] Run a secret-pattern scan and verify every referenced path, variable, domain, and command exists.
- [ ] Commit with `git commit -m "docs: document dual-production operations"`.

### Task 5: Deployment regression and fault-injection verification

**Files:**
- Modify: `.ccg/tasks/design-new-api-cicd/review.md`

**Interfaces:**
- Consumes: Tasks 1-4 and the application-branding plan.
- Produces: review evidence for development, Molii production, iXiaozu production, and partial failure.

- [ ] Run deployment tests, shell syntax, Compose rendering, workflow parsing, frontend checks, root Go tests, and relaykit tests.
- [ ] Simulate Molii success plus iXiaozu failure and assert Molii is not rolled back or cancelled.
- [ ] Simulate iXiaozu success plus Molii failure and assert iXiaozu remains deployed.
- [ ] Inspect the complete diff for shared secrets, hard-coded credentials, mutable-image deployment, and accidental data synchronization.
- [ ] Record exact outcomes and the unavailable external CCG model-wrapper limitation in the review file.
