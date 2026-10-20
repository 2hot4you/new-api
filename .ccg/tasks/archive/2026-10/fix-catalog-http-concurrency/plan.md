# Plan and evidence

Same SHA387937489 passes development and fails main in TestCatalogSyncHTTPConcurrentProofAndExactBinding. Uncontrolled local count10 passed; deterministic GORM callback interleaving reproduced the exact three failures: A503 CATALOG_SYNC_UNAVAILABLE, B403 SECURITY_PROOF_CONSUMED, zero successes/operations, wrong-digest lookup404 rather than409.
A pauses before proof UPDATE, B pauses inside catalog_sync_plans precheck holding writer; A continues and consumes proof but cannot acquire writer; B resumes and finds consumed proof. Production explicitly burns proofs even on action failure and uses fail-fast writer acquisition. This is an invalid test scheduling assumption, not grounds to weaken guards.

Repair only existing HTTP test: rendezvous both requests before real auth_flows UPDATE, after both prechecks. Race actual proof consumption; require exactly one200 and one403 SECURITY_PROOF_CONSUMED, one durable operation, unchanged digest conflict checks. Bound rendezvous and drain goroutines before removing callback/fixture cleanup. No sleeps/retries/production changes.

Validate focused repeated normal/race, full HTTP group normal/race, complete PostgreSQL normal acceptance. Independent reviewer before commit. Archive records and submit develop PR; remote CI covers complete normal/race and frontend.
Security references read: OWASP Authentication, Session Management and Transaction Authorization cheat sheets (server-side transaction binding, unique single-use authorization). No auth implementation changed, no claim of full ASVS compliance.
