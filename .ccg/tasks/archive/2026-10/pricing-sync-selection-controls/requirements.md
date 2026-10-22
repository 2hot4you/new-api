# Approved bounded UI change

User: 看不到，给我加上选择和全选功能。

Existing upstream price-sync checkboxes are technically visible but very low contrast, unlabelled visually, and inside the source-price column. Make per-source model selection explicit with visible Select/Clear selection text and a larger high-contrast checkbox. Make source-header bulk selection explicit with Select all/Clear selection text; retain selection of all currently filtered differing models across pagination. Reuse existing Checkbox and Label; do not change global checkbox styling, pricing amounts, backend contracts, environment catalog-sync scope, or the existing apply/confirmation flow. Do not apply pricing, push, merge, deploy or mutate production.

Use the already isolated worktree and new fix branch, preserving earlier diagnosis and catalog-contention work. Follow TDD and extend existing pricing-sync tests for text-label clicks, keyboard/focus, source exclusivity, disabled state and filtered bulk selection. Two independent read-only analyses/reviews substitute for the unavailable external antigravity/Claude wrapper; do not claim external dual-model verification.
