# Task 2 independent frontend review

## Scope and basis

Native independent review substituting for the unavailable external wrapper; no external-model review is claimed. Reviewed the immutable three-file `task-2-review.diff` against `a8e31cc44`, the task brief/report, approved requirements and plan Global Constraints. Inspected the concrete surrounding parser, save producer/consumer, existing visual removal and reset paths. No product, index, HEAD or branch changes; no broad test rerun.

## Spec Compliance — PASS

- `web/src/features/model-pricing/api.ts:256`: removal is confined to explicit `has_metadata === false`, nonempty original configured pricing, empty complete resulting pricing and absence of an own currency override. Missing entries, undefined eligibility, additions, edits, partial deletions and deletion with explicit currency intent still become blockers.
- `web/src/features/model-pricing/api.ts:263`: literal removal payload retains the snapshot version and contains only identity, expected version, empty pricing and `remove: true`; neither currency nor reset is manufactured. The Go JSON field name matches the frontend intent. Backend authority/transaction correctness is outside this task review.
- `web/src/features/model-pricing/api.ts:274`: eligible metadata-backed callers retain the existing ordinary payload. The early dedicated Seedance exclusion and separate metadata-gated Reset flow remain unchanged.
- `web/src/features/model-pricing/api.ts:244`: eligibility uses stored `configured`, not display-only `effective`; unchanged builtin display fields do not themselves constitute a dirty removal or get promoted into the remove payload. Complete-result checking retains non-dirty configured fields, so deleting only a visible subset cannot activate the exception.
- Existing visual removal deletes the model's fields and provider keys through shared controls. `ratio-settings-card.tsx:512` builds the complete batch before either pricing PATCH or visibility-option write; a blocker throws before either request, preserving atomic preflight.

## Quality — PASS

Critical: none.

Important: none.

Minor: none.

Strengths:

- Narrow production change in the shared payload builder; no new UI, strings, dependencies or duplicated removal controls.
- Unit assertions use exact literal payloads, original versions and absent currency; provider-only identities containing `::`, zero-valued overrides, original-empty/remaining-hidden-config guards and explicit same/undefined currency intent are covered.
- The formerly rejected explicit-false deletion is replaced by targeted allowed-removal coverage while the undefined-metadata rejection remains. Existing edit/add safeguards are not weakened.
- `web/src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx:370` drives actual existing edit/menu/remove/save controls for both qwen identities, checks PATCH and success, and verifies failed saves retain the deletion draft through an identical retry.
- `web/src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx:438` checks a mixed valid removal plus invalid edit rejects before both PATCH and option PUT, retaining the JSON draft.

## Verification and uncertainty

The frozen-source report records focused tests 44/44, typecheck, scoped lint/format and diff checks passing, with relevant pre-implementation failures. These are worker-reported results, not independently rerun here; the coordinator owns full-suite verification. Component tests mock only the network boundary and do not establish actual backend persistence/runtime behavior, which remains subject to Task 1 and integrated review. No correctness issue was found in the bounded frontend diff.
