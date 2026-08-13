# Review

## Result

- 固定价格与动态分档 Token 模型的价格表底部新增统一尾注。
- 尾注文案为 `在线推理 · ¥ / 1,000,000 Token`，视觉结构与 Seedance 一致。
- 未恢复表格上方的冗余计费说明。
- 原价格、表头、分档和底部模型元信息保持不变。

## Verification

- 回归测试先因尾注缺失而失败，再随实现转为通过。
- `bun test src/features/pricing`: 42 passed, 0 failed.
- `bun run typecheck`: passed.
- Scoped `oxfmt --check`: passed.
- Scoped `oxlint`: passed.
- `git diff --check`: passed.
- 本地 `/pricing` 页面确认统一尾注共 9 处，旧计费说明为 0 处。

按用户要求未调用 antigravity 或 Claude，未执行生产构建。
