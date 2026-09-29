# Review

## Scope

- Relabel the compatibility output-token field as image output for GPT Image 2 models.
- Keep the persisted billing expression on `c`; do not change backend settlement.
- Hide the redundant `img_o` option when it is not already present, while keeping existing legacy expressions editable.

## Findings

- Critical: none.
- Warning: none.
- Info: model detection is intentionally limited to the GPT Image 2 model family that uses the built-in `c`-based price expressions.

## Verification

- Regression test observed failing before implementation and passing after implementation.
- Affected test file: 36/36 passed.
- Full frontend suite: 283 files, 2695 tests passed.
- TypeScript typecheck passed.
- Changed-file lint passed.
- Production frontend build passed.
- `git diff --check` passed.

External antigravity/Claude review was not used, following the user's explicit instruction.
