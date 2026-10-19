# Release integration review

## Integrated changes

- Merged fix/seedance-adaptive-duration (implementation c5a2f2cb5) into origin/develop.
- Merged the complete, already published origin/codex/image-result-proxy branch at f57e46b67, including encrypted image links, strict public proxy protection, streaming accounting fixes, image API documentation and the image gateway timeout template.
- Both merges completed without conflicts. Prior main and develop histories are retained; no force push or other worktree reset was used.
- Application release source: 38793748924f176fc680a7a2f5ddb8b683ddb586.

## Fresh verification on merged code

- Complete Seedance-related package tests passed: relay/common, relay/channel/task/bytedanceseedance, relay/channel/task/starai and relay/channel/task/seedanceprotocol.
- Focused image proxy tests passed for service, controller, router, middleware, relay and relay/channel/openai.
- The six-package image proxy regression selection also passed with -race.
- go vet passed for the four Seedance packages and six image-path packages.
- git diff origin/develop..HEAD --check passed before publishing.
- Independent integration reviewer found no code-level merge regression. The image implementation files are identical to the reviewed source branch, and image changes do not overlap shared Seedance validation.
- Full-suite limitations from the preceding Seedance task remain recorded there. This task does not claim the historical broad cross-engine suite passed on current catalog code. GitHub's PostgreSQL workflow remains the release gate.
- CCG's external antigravity/Claude wrapper is unavailable; independent fallback review is not represented as the prescribed dual external review.

## model.claudeye.com deployment preservation

Read-only verification confirmed both IMAGE_RESULT_PROXY_ENABLED=true and IMAGE_RESULT_PROXY_BASE_URL=https://model.claudeye.com are already in the persistent runtime file, and both existing crypto/session secrets are present. Secret values were not emitted. The Nginx configuration test passed.

The one-off rollout had left .env.runtime and .deploy.env owned by root with mode 600, preventing claudeye-deploy from reading them for the normal workflow. Within the user-authorized release preparation, preserved exact content backups and restored ownership to the existing deployment UID/GID 1000:1000 while retaining mode 600. Exact content comparison against backups passed. docker compose --env-file .deploy.env config --quiet passed afterwards. Running application PID remained 403792.

Backup directory on the server: /opt/claudeye/production/backups/cicd-runtime-ownership-20261019T090524Z. Backups preserve the prior ownership and contents. No application/container restart was performed during preparation; the upcoming requested deployment may replace its application container normally.

Existing gateway timeout and download configuration are outside workflow replacement and remain installed.

## Remote delivery

Both remote develop and main were verified at application release source 38793748924f176fc680a7a2f5ddb8b683ddb586 after normal fast-forward pushes. All source branch tips are ancestors of that release.

- Development CI/CD: https://github.com/2hot4you/new-api/actions/runs/39227556806
- Production CI/CD: https://github.com/2hot4you/new-api/actions/runs/39227619885

Both runs were in progress when inspected. Production workflow resolves the four configured environments: production-molii, production-ixiaozu, production-claudeye and production-model-claudeye. Starting the workflow is not confirmation of successful deployment.

The archive-only follow-up commit touches .ccg/tasks only and is ignored by the application deployment workflow; it does not replace the verified application release source.
