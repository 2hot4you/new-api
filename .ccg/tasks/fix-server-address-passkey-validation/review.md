# Independent review2026-11-02

Two independent native analysts and two fresh native reviewers were used; the external antigravity/Claude executable is unavailable, and no external dual-model result is claimed.

Security reviewer (gpt-6-astra): no Critical/Important/Minor. Confirmed only automatic retention changes; explicit RP/origin validation, root route authorization, transactional preview rollback, known/unknown credential counting and freshness-bound removal confirmation remain unchanged. Independently ran SQLite Passkey tests and Python gates. Declined real-engine/race/live deployment assessment; primary executor must verify those separately. Unrelated changes not reviewed.

Release reviewer (gpt-6.1-sol): no Critical/Important. Confirmed PostgreSQL selection is real and required in both modes; main integration keeps app/docs aigc exclusion, and main-only ce7 has no tree delta against merge-base e32. Declined full rc47/OWASP compliance audit and future refs/deployment assessment; existing rc47 review/acceptance is recorded in its archived task, and source refs/live state must be freshly checked before promotion.

Minor notes: positive old-domain retention is run locally on all three engines but not added to this fix's new required group; broader positive coverage is a possible follow-up. Existing FAILED_REGRESSIONS comment mentions a historical run although this change adds new required regression names; no runtime or gate weakening. No production expansion requested.

One proposed minor (raw localhost:3000 allegedly fails before origin validation) was corrected and retracted by the reviewer: validation uses effective.EffectiveRPID(), whose net.SplitHostPort strips the port. The existing explicit-invalid test reaches origin mismatch as intended; no duplicate test or production change is warranted.

Applicable security guidance: [ASVS5.0.0](https://asvs.owasp.org/), [Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html), [Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html), [Passkey Security](https://cheatsheetseries.owasp.org/cheatsheets/Passkey_Security_Cheat_Sheet.html). This narrow review preserves RP/origin binding, server-side controls and credential lifecycle safeguards; it is not a complete compliance assessment.
