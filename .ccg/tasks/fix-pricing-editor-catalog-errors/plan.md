# Pricing editor and catalog error repair

Authority: requirements.md plus approved in-chat bounded design. This plan is required by project L-size coordination; it does not authorize a new pricing system or relaxing billing guards.

## Global Constraints

- No live price/metadata/config changes, catalog apply/restore, restarts or paid calls. Only normal authorized CI/CD deployment through branch pushes.
- No fixed() task-usage acceptance, model-name exemptions, guessed quotes/currency/metadata, disabled plugins or blanket raw backend error exposure.
- Preserve server metadata/currency/version/security fences and dedicated Seedance/Grok pricing; preserve drafts and atomic save semantics.
- Reuse existing project UI/error/dialog/form primitives. Every added UI string translated in all seven locales; no new dependencies/docs/ files.
- All workers share this checkout; never revert others, never spawn children, never push or merge. File ownership is exclusive, locales reserved for Task3.

## Task 1: Pricing editor and save affordances

Owner: frontend implementer. Allowed: web/src/features/model-pricing/**; web/src/features/system-settings/models/**. Excludes catalog-sync/** and locales. Read both AGENTS, shadcn skill, existing primitives, related tests. Write failing behavior tests before production changes.

- Distinguish metadata prerequisite from real pending in desktop/mobile/embedded shared editor. Provide truthful disabled reason and safe navigation to existing exact metadata management, without implicit metadata writes.
- New name eligibility/preflight must be explicit. JSON missing-metadata edits must not be silently discarded or mixed-success misreported; reject before request with actionable names and retain draft. Existing known model with valid metadata saves normally.
- Keep a visible apply/save action for local deletions even when selected/last editor closes; distinguish removing overrides/defaults. Filter reset-all to eligible persisted overrides or reject clearly without partial mutation. Preserve dedicated Seedance exclusion and valid non-Seedance behavior.
- Add focused regression coverage for metadatafalse/no fake pending, truepending/failure, missing-only/mixed JSON, add eligibility, deletion selected/last row, default-only reset behavior. Keep scope narrow; report translation keys for Task3.
- Run affected tests/typecheck/scoped lint; commit only owned paths. Report exact red/green evidence and concerns to task1-report.md.

## Task 2: Typed catalog validation conflict feedback

Owner: catalog implementer. Allowed: model/catalog_sync_validate.go, model/catalog_sync_apply.go only as necessary for wrapping; controller/catalog_sync.go and existing catalog controller tests; web/src/features/system-settings/catalog-sync/**. One new backend test file max if no suitable existing case. Excludes all Task1 paths and locales.

- Keep deterministic prospective pricing rejection fail-closed, but project a typed safe validation conflict with model/provider identity and allowlisted/localizable reason to root management caller; unknown DB/compiler/runtime errors remain generic and never expose expressions, SQL, credentials or raw error text.
- Prefer structured error wrapping at exact validation origin and errors.As in existing HTTP projection, not regex-parsing arbitrary text. Include real Grok fixed/task usage reproduction, metadata boundary where applicable, and unknown-error redaction tests.
- Show actionable validation conflict in existing catalog UI without misleading ready/selectable success, apply availability or silent source/target changes. Preserve existing transport/config error distinctions. Do not change auth or plugin runtime/schema/DB behavior.
- Run focused Go/frontend tests and scoped checks; report any true DB behavior effect requiring three-engine verification rather than assuming. Commit owned paths only; translation keys to Task3. Report to task2-report.md.

## Task 3: Translation/integration verification and review

Owner: integration worker after Tasks1/2. Allowed: web/src/i18n/locales/*.json and own report; no unsolicited app fixes. Add precise translations for new keys preserving all existing values. Check linked UI/error contracts from reports; notify coordinator of gaps.

Run frontend full suite, typecheck, lint affected files, i18n completeness, production build; Go affected suites and repository required PostgreSQL gate if changed path demands it. Record exact commands/output/log files and any failures by name. Two independent final reviewers assess full application diff and security/billing boundaries. Fix findings through owning worker and re-review; no approval bypass.

## Task 4: Verified branch release

Coordinator owns task/checkpoint/spec feedback and git release. Fetch/check remote branches, merge only reviewed application commits on verified base, no force pushes or unrelated local audit commits. Push develop; inspect explicit --repo 2hot4you/new-api exact SHA jobs/attempts, including verify and actual development Release job. Check public development health/version matches actual source SHA and functional non-mutating response/visible states where authenticated access permits. Never replay a price write to manufacture acceptance.

Only after exact-source develop CI/CD green, fresh fetch detects no unverified intervening changes, merge/push main. If CI failure diagnose/minimally fix/test/review/re-push. Do not weaken checks, blindly replay migrations or claim health proves all edit/apply paths. Persist actual SHAs/run links/status and archive only when authorized release criterion reached. No stale prior automation resurrection; use existing workflow monitoring mechanism if a later wakeup is needed.
