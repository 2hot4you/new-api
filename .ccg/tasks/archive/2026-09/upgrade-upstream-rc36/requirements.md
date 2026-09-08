# Requirements — upgrade upstream rc.36

## Goal

Audit and integrate upstream QuantumNous/new-api releases from v1.0.0-rc.31 through v1.0.0-rc.36 into the Molii fork currently based on rc.30, preserving Molii-specific behavior and fixing merge or behavioral conflicts.

## Constraints

- Work in an isolated branch/worktree and do not overwrite the active CI/CD task.
- PostgreSQL and Redis are the supported production data stores; SQLite is not an acceptance target.
- Preserve Molii pricing, model metadata, group routing, temporary assets, task plugins, usage logs, documentation, and admin UI customizations unless an upstream replacement is explicitly adopted.
- Review every intermediate release and its database/configuration compatibility notes.
- Exercise PostgreSQL migrations on a safe development copy before any production recommendation.
- Do not deploy production or push/merge until the user approves the migration design and implementation result.
