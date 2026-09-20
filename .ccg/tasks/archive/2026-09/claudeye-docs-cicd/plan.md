# Claudeye documentation deployment

Approved scope: https://claudeye.com/docs and https://model.claudeye.com/docs. Shared source and claudeye branding, environment-specific API and console origins. Initial deployments explicitly target one site from main. Existing main automatic targets remain unchanged. No blue-green or business/database changes.

1. Extend docs workflow with explicit manual target selection and branch/target validation. Preserve existing push path filters and environments. Derive summary count from resolved targets.
2. Extend docs-site/src/config.ts brand allowlist for claudeye and add origin/brand contract tests. Reuse approved placeholder assets without runtime-secret exposure.
3. Extend docs-site/deploy/deploy.sh with isolated Claudeye environment and public-directory mappings. Preserve lock, checksum, archive traversal checks, snapshot rollback and health checks; add contracts for both new targets.
4. Document GitHub DOCS_* environment values, static directory ownership and narrow OpenResty /docs locations. Inspect existing site mounts before proposing concrete server commands. Never replace whole 1Panel site config.
5. Run documentation tests, build both origins, link/content/secret checks, deployment contracts and ShellCheck/actionlint. Review and create develop PR. Production merge and initial docs release remain separate stages.

Analysis blocker: required antigravity and Claude parallel commands were both attempted; ~/.claude/bin/codeagent-wrapper does not exist. No callable equivalent discovered. No application/workflow/server changes have been made.

User subsequently approved independent review and tests. Code and local validation complete; production setup and acceptance remain pending.
