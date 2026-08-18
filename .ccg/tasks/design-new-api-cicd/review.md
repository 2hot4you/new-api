# Review: schema-safe deployment version

## Finding

`development-<40-character SHA>` is 52 characters, while `setups.version` is `varchar(50)`. The setup request created the root user and options before the final setup record insert failed with SQLSTATE 22001.

## Fix reviewed

- Generate the application-visible version from the environment name and a 12-character commit SHA.
- Keep the full SHA in immutable image tags and OCI revision metadata.
- Exercise the real version generator in the deployment contract test and assert that its output fits the database schema.

## Verification

- `bash deploy/tests/deploy_test.sh`: 39/39 assertions passed.
- `bash -n deploy/app-version.sh deploy/tests/deploy_test.sh`: passed.
- GitHub workflow YAML parsed successfully with Ruby YAML.
- `git diff --check`: passed.

## External review availability

Both configured CCG review backends were invoked, but the local wrapper `~/.claude/bin/codeagent-wrapper` is not installed. No external model review result was available; local code, contract, and runtime evidence were used instead.
