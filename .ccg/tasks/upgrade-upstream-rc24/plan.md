# v1.0.0-rc.24 staged upgrade plan

## Preparation

- [ ] Confirm branch, status, remotes, and current commits.
- [ ] Push the existing `feat/molii-auth` history to `origin`.

## Batch 1 — items 1 + 2

- [ ] Add failing concurrency and route-limiter tests.
- [ ] Port access-token and affiliate accounting atomic updates.
- [ ] Wire user-keyed critical rate limits to token rotation and affiliate transfer.
- [ ] Run focused and full backend tests; commit.

## Batch 2 — item 3

- [ ] Add failing replayable body, HTTP/2 replay, redirect, and task-body tests.
- [ ] Port upstream replayable-body implementation.
- [ ] Resolve `relay/image_handler.go` manually and preserve all Molii Grok behavior.
- [ ] Run focused relay/Grok/Seedance and full backend tests; commit.

## Batch 3 — items 4 + 5 + 6

- [ ] Add/port failing redemption precision, native channel-test, and model-category tests.
- [ ] Port the three upstream improvements without unrelated release workflow files.
- [ ] Run focused Go/frontend tests plus type/lint/format checks; commit.

## Completion

- [ ] Run final verification and inspect the full rc.24 diff.
- [ ] Archive the CCG task.
- [ ] Push `feat/molii-auth` to `origin` without creating a PR.
