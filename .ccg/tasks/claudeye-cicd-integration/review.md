# Review and validation

Changes: manual-only new production targets, immutable source resolution, existing main push matrix retained, scoped local-DB Compose with nonroot1000, no orphan removal, preflight does not recreate, public rollback health, rollback-aware redacted notifications, independent brand profile and runtime examples.

Validation passed: 169 deploy contract assertions; 8 Python tests including real temporary Git source resolution and runtime preparation; 6 brand tests; frontend typecheck; Molii production build; explicit Claudeye fixture production build (fixture icons not publishable assets); GOWORK=off go test ./... root and relaykit, relaykit independent go build ./...; actionlint1.7.7; full ShellCheck0.11 with source following and intentional literal-expression suppression; git diff check. Real Docker Compose config rendering checked both site projects, nonroot, localhost3000, application-only service and two networks, no proxy injection. No actual new application launched or DB migrations exercised by this change.

Initial concurrent root Go test failed because a parallel web build removed embedded dist; after frontend build completion a serialized rerun passed. No code failure hidden.

Independent read-only agent review found no actionable issues within this two-host scope. Numeric UID/GID1000 verified via SSH on both servers; runtime generator uses actual account ownership. Image rollback does not undo schema or restore previous Compose file; preserve compatible changes and only single-container downtime expected.

Required antigravity+Claude external analysis and review were both attempted in parallel; wrapper absent, so named dual-model review remains UNCOMPLETED. Do not represent agent review as that requirement being fulfilled.

Operational state: both TLS endpoints verified HTTP502 (app absent), Docker29.5.0/29.5.2 and Compose5.1.3/5.1.4 accessible, new GitHub environments branch policies main only. Only3deployment vars exist per environment; 9brand variables still needed. No push/merge/GitHub dispatch or app deployment. Newmain workflow first requires develop acceptance and explicit authorization of existing main production release.

2026-09-20 branding follow-up: user selected shared lowercase claudeye name and authorized temporary assets. Added authored C geometry SVG,32px PNG favicon,180px PNG Apple Touch icon and public variable manifest. PNG visually inspected, formats verified. All6brand tests and production build with exact manifest passed; generated HTML title/iconreferences and copied asset bytes verified. Both new GitHub Environments8brand vars written/readback matched, other variables preserved. No Secrets or other environments changed, no dispatch/push/merge/server actions. Named dual analysis/review attempted again; wrapper unavailable.
