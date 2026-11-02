# Narrow server-address save repair

The actual authenticated production UI shows empty server address, primary RP, legacy RPs and origins; Passkeys and insecure origins are disabled. Public status derives localhost from the runtime default. Saving only ServerAddress routes through passkey-domain rotation, materializes the unusable HTTP default as retained trust, then rejects it. The UI is not sending a URL as an RP ID.

Repair only automatic retention of implicit previous defaults: retain an old implicit origin/RP only if it was usable under the existing RP syntax and origin security rules. An unusable implicit default must not become explicit trust. Keep explicit settings validation, credential impact/confirmation, transaction atomicity and valid old-domain retention. Do not enable insecure origins, ignore validation merely because login is disabled, or rewrite explicit historical RP values. No frontend rewrite; status invalidation already refreshes /keys.

Compatibility clarification from full-suite regression: always materialize the newly requested origin on first setup exactly as before; only omit the unusable OLD origin. Leaving the new origin implicit would allow a later ServerAddress save to expand trust silently. Preserve the pre-materialization explicitOrigins flag so initial IP/non-WebAuthn setup remains unchanged. Existing TestCatalogMixedPasskeySave must remain unmodified and pass.

Alternatives rejected: bypass all passkey checks (weakens trust/removal controls), turn on insecure origins (unrequested security weakening), hardcode browser origin in keys page (hides failed server configuration). Implementation choice follows the user's specified bug-fix scope, not a new feature.

Security references read: OWASP ASVS stable5.0.0 overview; Authentication and Session Management Cheat Sheets plus Passkey Security guidance. Applicable controls: enforce RP/origin validation server-side, retain root authorization, preserve freshness-bound credential-removal confirmation, never log secrets. No claim of full ASVS certification.

Release: develop acceptance must include full Verify and real development deployment exact SHA. Main promotion includes already development-accepted rc47 changes and deployment maintenance guards. Preserve main's divergent merge ancestry using a normal reviewed merge. Default main release targets only molii/claudeye/model-claudeye. Aigc remains9a; requires separately dated approval and20:00–23:59 Beijing maintenance, not part of this batch.
