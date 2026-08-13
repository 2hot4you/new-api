# Review: Model overview capabilities redesign

## Scope

- Replaced the fragmented generic LLM quick-stat and capability blocks with one integrated capability card.
- Kept pricing, Performance/API tabs, and Seedance/Grok specialized overview components unchanged.
- Moved models.dev provenance into a single low-priority footer and removed duplicate provider-card fields.

## Findings

- Critical: none.
- Warning: none.
- Info: the core-specification grid now adapts to one, two, or three populated facts so missing release metadata cannot leave a blank column.

## Verification

- `bun run typecheck`: passed.
- `bun test src/features/pricing`: 54 passed, 0 failed.
- Scoped `oxlint`: passed.
- Scoped `oxfmt --check`: passed.
- `git diff --check`: passed.
- Local `GET /api/status` and `GET /pricing`: HTTP 200.
- Browser verification at `http://127.0.0.1:3000/pricing`: one capability card, two populated specification cells, one modality row, five capability items, and no duplicate provenance in provider information.

External antigravity/Claude review was intentionally not invoked because the user explicitly prohibited those models. No sub-agent was started for this task.
