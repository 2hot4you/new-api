# Protected default API key report

## Self-service creation paths and transaction boundaries

- Password registration (`controller/user.go:Register`) now calls `model.CreateSelfRegisteredUser`.
- Standard OAuth registration (`controller/oauth.go:findOrCreateOAuthUser`) calls `InsertWithDefaultTokenTx` inside its existing transaction, before either OAuth binding or provider-ID update. Existing `FinalizeOAuthUserCreation` continues only after commit, preserving sidebar/invite-reward behavior.
- WeChat first registration (`controller/wechat.go:WeChatAuth`) now calls `model.CreateSelfRegisteredUser`.
- The shared helper (`model.User.InsertWithDefaultTokenTx`) inserts the user then `CreateDefaultTokenWithTx` in one transaction. Failed key generation/insert returns the error and rolls back the user. The helper is idempotent in a transaction and does not promote an existing ordinary key.
- Administrator creation remains on `InsertWithTx` (the controller management path); bootstrap/admin initialization and existing users are untouched.
- `GENERATE_DEFAULT_TOKEN` is no longer read by password registration. It remains as a compatibility setting but cannot cause a self-registered user to miss its protected key.

## Schema and migration

- `Token.IsDefault` is `is_default`, non-null, default false, and is included in normal JSON token responses.
- GORM migration picks the field up through the existing `Token` AutoMigrate registration in `model/main.go`.
- `deploy/migrations/20260812_default_token.sql` is PostgreSQL-15 idempotent: it adds `is_default` for existing `tokens` tables and creates the partial unique index `ux_tokens_one_live_default_per_user` on `(user_id)` where `is_default = true AND deleted_at IS NULL`.
- Compose runs this migration last, after the existing 20260803/20260810/20260811 files, and mounts it read-only. The deployment README documents the order and contract test.

## API behavior

- Normal self-service default key preserves the established generated key format, Auto-group default, unlimited quota, expiry, and initial quota semantics.
- Default keys remain editable, status-toggleable (including disable), and key-readable/reset-capable through existing non-delete paths.
- Model-level single delete and batch delete reject default keys. Controller responses use HTTP 200 compatibility envelope with stable `code: "default_token_delete_forbidden"`.
- Batch deletion loads candidates, rejects before delete if any protected key appears, and rolls back: ordinary selected keys are retained. Ordinary-only deletion remains unchanged.
- `POST /api/token/:id/rotate` requires `{ "confirm": true }`, supports every owned key including the default key, atomically replaces its secret, invalidates the old Redis token cache after commit, and returns only `{ "key": "<new raw key>" }`. The old key no longer resolves; generation or database failure leaves the old key untouched.

## Frontend behavior

- `ApiKey` reads `is_default` (legacy omitted values remain false).
- Default keys display a `Default` badge, have no row-selection checkbox, cannot be selected by select-all/table row-selection policy, and omit the delete action. Backend protections remain authoritative.
- Every row, including a default key, has a Rotate Key action that opens an explicit destructive confirmation dialog. On success the new key replaces the client-side resolved-key cache so it can be copied through the existing key UI.
- The UI policy test asserts that a default key shows Rotate while omitting Delete; rotate requests always send `confirm: true`, and raw/previously-prefixed returned keys normalize to exactly one `sk-` display prefix.

## Tests and validation

- PASS `go test ./model -run 'Test(CreateSelfRegisteredUser|CreateDefaultTokenWithTx|TokenAutoMigrateAddsDefaultFlag|DefaultToken|BatchDeleteDefault)' -count=1`
- PASS `go test ./controller -run 'Test(PasswordRegistrationCreatesDefaultToken|OAuthRegistrationCreatesDefaultToken|WeChatRegistrationCreatesDefaultToken|DefaultTokenResponsesExposeFlagAndRejectSingleAndBatchDelete)' -count=1`
- PASS `go test -race ./model ./controller -run 'Test(CreateSelfRegisteredUser|CreateDefaultTokenWithTx|PasswordRegistrationCreatesDefaultToken|OAuthRegistrationCreatesDefaultToken|WeChatRegistrationCreatesDefaultToken|DefaultTokenResponsesExposeFlagAndRejectSingleAndBatchDelete|BatchDeleteDefault)' -count=1`
- PASS `go vet ./model ./controller`
- PASS `./deploy/migrations/20260812_default_token_test.sh` (idempotence, boolean/default column, partial unique index, soft-delete replacement)
- PASS `web: bun run typecheck`
- PASS `web: bun test src/features/keys/lib/__tests__/default-api-key.test.ts src/features/keys/lib/__tests__/auto-group-form.test.ts` (11 tests)
- PASS `git diff --check`
- PASS `go test ./model ./controller -run 'TestRotateTokenKey' -count=1` (default allowed, ownership, old/new validity, generated-key failure, database failure, confirmation/minimal response)
- PASS `web: bun test src/features/keys/lib/__tests__/default-api-key.test.ts src/features/keys/lib/__tests__/rotate-api.test.ts` (default action policy, prefix normalization, confirmed rotation payload)

## Residual risk

- The partial unique constraint is explicit for PostgreSQL deployment. GORM adds the boolean field for SQLite/MySQL/PostgreSQL; the application helper prevents duplicate creation per new user, while the PostgreSQL deployment migration provides the durable live-row uniqueness rule needed by production.
- No production build, commit, or push was run. Unrelated pricing/model-catalog changes already present in the shared worktree were preserved.
