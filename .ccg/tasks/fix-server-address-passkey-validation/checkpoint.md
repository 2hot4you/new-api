# Progress2026-11-02

Branch fix/server-address-passkey-validation from fresh origin/develop1f7724f74d047eeef14078697ef8d54479c7cdeb. Main ce7c4eb362728f3d55c72e80255f36829483b0a9. Reused linked worktree; pre-existing deleted task files and duplicate/untracked files untouched.

Fresh public status success at five sites: dev development-a85a9daa5ec1; molii/claudeye/model-claudeye ce7c4eb36272; aigc production-ixiaozu-9a27a34a4bf3. Model site returns server_address http://localhost:3000, passkey_loginfalse, effective RP localhost. Authenticated browser read-only inspection confirms stored UI fields ServerAddress/primaryRP/legacyRP/origins empty, insecurefalse. No production form saved or modified. Rootcause automatic retention of an unusable implicit HTTP bootstrap origin/RP. Not a URL typed into wrong field, no hardcoding needed.

Independent native backend and frontend/release investigators agree. CCG external wrapper missing; no external model review claimed. Existing focused passkey baseline passes. Main currently lacks new guards; never dispatch old main all-production. New develop gates exclude aigc from main push for apps/docs; standalone requires separate same-date Beijing20:00+approval.

Inline execution per user CCG S-M rule; bug-fix desired behavior fully specified, no new UI/product design. Ruling: preserve rejection of explicitly configured invalid historical RP/origins instead of silently rewriting them; actual model-site empty configuration is covered. No deployment/restart/SQL changes or paid calls yet.

## Reviewed candidate

Owned code/config diff: model/passkey_option.go (13-line narrow retention change), existing controller/passkey_test.go (three compact regression functions), explicit normal/race PG gate inventory and required regression list. No frontend or workflow changes. New tests observed RED at exact original invalid-domain error; GREEN on SQLite3.50.4, MySQL8.4.11, PostgreSQL15.18. Full selected Passkey suites25top-level tests per engine, focused PG race25PASS95.314s, frontend API addresses5PASS, Python22PASS, go vet PASS. Two independent fresh reviewers: no Critical/Important; disputed raw-port minor was corrected/retracted.

Local make test-postgres normal is still running (session97422, log .superpowers/sdd/plan-fix-server-address-passkey-validation/postgres-ci-normal.log). Complete audited root43packages/2733tests, relaykit9/583, tokenkit1/7 already PASS; mandatory PG groups in progress with no failure observed. Unfiltered controller run was stopped, not claimed passing. Ruling: push a reviewed regression-tested candidate while local broad acceptance continues to avoid serial duplication of the requested clean remote CI; no main merge or completion claim until clean remote full normal/race/frontend Verify and exact development version. If local suite discovers a real failure, stop promotion and investigate.

Pre-push fresh fetch still origin/develop1f7724f74d047eeef14078697ef8d54479c7cdeb, origin/maince7c4eb362728f3d55c72e80255f36829483b0a9. Main is same tree as merge-basee32; normal integration must preserve full accepted develop tree and aigc gates. Do not dispatch old-main all-production.
