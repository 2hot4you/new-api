# Local merge audit: upstream rc.36

## Frozen revisions

- Molii target: `origin/develop` at `e4120574b69e7dad17da278e246aabe5a6a0edb7`
- Upstream target: `v1.0.0-rc.36` at `ea7cb0ba4e0f82e2bfa5e55752eb68bdf902f71b`
- Common ancestor: `v1.0.0-rc.30` at `27ff6a8767e728f879d52770c273d4f73214a430`

## Scale

- Molii changes since rc.30: 544 commits, 1,608 changed paths.
- Upstream changes since rc.30: 56 commits, 831 changed paths.
- Paths changed by both sides: 149.
- Virtual merge conflict messages: 77.
- Unique conflict paths after normalizing modify/delete messages: 73 (one merge-tree summary token excluded).
- Frontend conflict paths: 43 by top-level inventory; the detailed frontend audit identifies 46 content/modify-delete conflicts when generated/shared web paths are included.

The merge simulation used `git merge-tree --write-tree --name-only --messages`; it did not update the branch, index, or worktree.

## Conflict concentration

- Model/vendor metadata and pricing: incompatible ownership and sync semantics.
- API keys: Molii ordered provider/group routing conflicts with upstream quota/table refactors.
- Usage logs: upstream audit and typed log metadata overlap Molii media previews and billing details.
- Authentication/security: upstream session-scoped verification proofs, PATs, Telegram OAuth and Argon2id must be adopted as one contract while preserving Molii registration defaults and branded UI.
- Task polling/billing: upstream failure settlement must not bypass Molii's durable terminal-task and billing-job transaction.
- Relay/model mapping: upstream canonical billing identity and `@` modifiers must coexist with Molii compact aliases and custom providers.

## Integration policy

1. Merge the release tag once to retain upstream ancestry; do not cherry-pick the release series or copy directories wholesale.
2. Resolve in dependency order: database/security contracts, polling/billing, relay/routing, model/vendor/pricing, keys/logs, routes/i18n.
3. Preserve Molii local catalog authority and durable settlement; selectively adopt upstream capabilities, safeguards and UI primitives.
4. PostgreSQL and Redis are the only acceptance data stores. SQLite is not an acceptance target.
5. Require disposable PostgreSQL/Redis migration, auth and fault-injection tests before any dev deployment.

## External analysis availability

The configured CCG wrapper was attempted for both `antigravity` and `claude`, but `/Users/naf/.claude/bin/codeagent-wrapper` is absent and both invocations exited 127. No external-model result is claimed. Two repository research agents independently audited frontend and backend/data risks instead; their reports are stored beside this file.
