# SDD ledger — plan: docs/superpowers/plans/2026-08-09-molii-channel-hardening.md

Base commit: 8e406f1e9e9c3b6c1898eb68228853ea0d9d7b29
Execution mode: shared current workspace because existing uncommitted Grok/generation-record changes are required context and must be preserved.

Task 1: fix round 1/5 (6 addressed, 0 open — lease fencing, redaction, UTF-8/NUL, internal idempotency key, primary key tag, lease index; commit 6e9d9b29)
Task 1: complete (commits 8a1e5d16, 6e9d9b29; spec ✅; quality approved)
Task 2: fix round 1/5 (6 addressed, 0 open — safe cache invalidation, zero-cost settle statistics, bounded debt, unlimited-token accounting, authoritative succeeded reload, concurrent/CAS rollback coverage; commit 187196f8)
Task 2: complete (commits 56e91ed1, 187196f8; spec ✅; quality approved)
Task 3: fix round 1/5 (3 addressed, 0 open — failure-relative retry clock, real claim/CAS concurrency, context-bound DB work; commit 7b47e5d5)
Task 3: complete (commits e82e77ed, 7b47e5d5; spec ✅; quality approved)
Task 4: fix round 1/5 (4 addressed, 0 open — safe missing-job state, channel-scoped polling identity, context-bound terminal transaction, surfaced query errors; commit d056288a)
Task 4: fix round 2/5 (4 addressed, 0 open — exported API compatibility, logger race, stable cancellation harness, scheduler query-error surfacing; commits 1a59b967, ddd4278a)
Task 4: complete (commits 4a159be2, d056288a, 1a59b967, ddd4278a; spec ✅; quality approved)
Task 5: fix round 1/5 (2 addressed, 0 open — cache-disabled real startup wiring and entry-level coverage; commits c3630c51, 7881ab08)
Task 5: complete (commits 3a1da182, c3630c51, 7881ab08; spec ✅; quality approved)
Task 6: fix round 1/5 in progress (self-contained preview retirement, pre-billing image mapping, requested/billed image snapshots; commit a7ca03de)
Task 6: fix round 2/5 (2 addressed, 0 open — final selected-channel consistency, type 61/62 no-switch/no-replay, race-safe fixtures and single-call regression; commit ccf228c6)
Task 6: complete (commits 1c7ebfe2, a7ca03de, ccf228c6; spec ✅; quality approved)
Preview retirement surface: fix round 1/5 (1 addressed, 0 open — Demo README; commit 74b73109)
Preview retirement surface: complete (commits 626eed55, 74b73109; spec ✅; quality approved)
