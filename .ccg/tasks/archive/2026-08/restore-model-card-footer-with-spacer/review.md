# Review

## Result

- 模型描述、计费说明与价格表保持连续展示。
- `default`、计费方式、端点和 Token 单位保留在卡片底部。
- 卡片的弹性剩余空间位于价格表与底部元信息之间。
- 未修改模型价格、计费表达式或接口行为。

## Verification

- `bun test src/features/pricing`: 42 passed, 0 failed.
- `bun run typecheck`: passed.
- Scoped `oxfmt --check`: passed.
- Scoped `oxlint`: passed.
- `git diff --check`: passed.
- 本地开发页 `/pricing` 已目视确认文本、Grok 与 Seedance 卡片顺序一致。

按用户要求未调用 antigravity 或 Claude，未执行生产构建。
