# Review

## Scope

- `/pricing` 分组筛选不再展示内部 `default` 分组。
- 模型详情、价格计算、账户权限和后端路由继续保留 `default` 数据。

## Verification

- TDD 回归用例先确认旧行为会返回 `default`，修复后通过。
- Pricing 测试：40 个文件、238 项测试通过。
- TypeScript 类型检查通过。
- 前端生产构建通过。
- 受影响文件的 Oxfmt、Oxlint 与 `git diff --check` 通过。

变更仅 11 行生产代码且风险低，按项目规则无需外部模型审查。
