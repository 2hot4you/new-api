# v1.0.0-rc.24 staged upgrade plan

## Preparation

- [x] Confirm branch, status, remotes, and current commits.
- [x] Push the existing `feat/molii-auth` history to `origin`.

## Batch 1 — items 1 + 2

- [x] Add failing concurrency and route-limiter tests.
- [x] Port access-token and affiliate accounting atomic updates.
- [x] Wire user-keyed critical rate limits to token rotation and affiliate transfer.
- [x] Run focused and full backend tests; commit.

## Batch 2 — item 3

- [x] Add failing replayable body, HTTP/2 replay, redirect, and task-body tests.
- [x] Port upstream replayable-body implementation.
- [x] Resolve `relay/image_handler.go` manually and preserve all Molii Grok behavior.
- [x] Run focused relay/Grok/Seedance and full backend tests; commit.

## Batch 3 — items 4 + 5 + 6

- [x] Add/port failing redemption precision, native channel-test, and model-category tests.
- [x] Port the three upstream improvements without unrelated release workflow files.
- [x] Run focused Go/frontend tests plus type/lint/format checks; commit.

## Completion

- [x] Run final verification and inspect the full rc.24 diff.
- [ ] Archive the CCG task.
- [ ] Push `feat/molii-auth` to `origin` without creating a PR.
