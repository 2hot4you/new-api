# Review

## Scope

- 使用统一的 `1px` 网格间隙增强模型卡片分隔。
- 移除单卡右侧和底部边框，避免边框重叠。
- 加深卡片悬停背景，但不改变卡片内容、排序或数据。

## Findings

- Critical: none.
- Warning: none.
- Info: 保持当前高密度连续网格，不引入阴影、圆角或额外卡片间距。

## Verification

- TDD RED: 旧网格缺少连续 `gap-px` 分隔，测试按预期失败。
- TDD GREEN: focused test 2 passed, 0 failed.
- Pricing component tests: 33 passed, 0 failed.
- `bun run typecheck`: passed.
- Scoped `oxlint`: passed.
- Scoped `oxfmt --check`: passed.
- `git diff --check`: passed.
